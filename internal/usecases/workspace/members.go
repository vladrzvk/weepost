package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/events"
)

// PlanLimitsChecker port local pour le contrôle des limites de plan (M-5).
// validate est déclaré dans workspace/create.go (même package).
type PlanLimitsChecker interface {
	CheckMemberLimit(ctx context.Context, workspaceID uuid.UUID) error
}

// ─────────────────────────────────────────────────────────
// InviteMemberUseCase
// ─────────────────────────────────────────────────────────

type InviteMemberCommand struct {
	ActorID     uuid.UUID `validate:"required"`
	WorkspaceID uuid.UUID `validate:"required"`
	Email       string    `validate:"required,email"`
	Role        string    `validate:"required,oneof=owner admin manager editor viewer"`
}

type InviteMemberResult struct {
	InvitationID string    `json:"invitation_id"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type InviteMemberUseCase struct {
	workspaceRepo  domain.IWorkspaceRepo
	invitationRepo domain.IInvitationRepo
	userRepo       domain.IUserRepo
	eventBus       domain.IEventBus
	planChecker    PlanLimitsChecker
}

func NewInviteMemberUseCase(
	workspaceRepo domain.IWorkspaceRepo,
	invitationRepo domain.IInvitationRepo,
	userRepo domain.IUserRepo,
	eventBus domain.IEventBus,
	planChecker PlanLimitsChecker,
) *InviteMemberUseCase {
	return &InviteMemberUseCase{
		workspaceRepo:  workspaceRepo,
		invitationRepo: invitationRepo,
		userRepo:       userRepo,
		eventBus:       eventBus,
		planChecker:    planChecker,
	}
}

func (uc *InviteMemberUseCase) Execute(ctx context.Context, cmd InviteMemberCommand) domain.Result[InviteMemberResult] {
	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[InviteMemberResult](domain.NewDomainError(domain.ErrCodeVALIDATION, err.Error(), nil, domain.SeverityLOW, false))
	}

	// ② Charger le workspace
	_, err := uc.workspaceRepo.GetByID(ctx, cmd.WorkspaceID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Fail[InviteMemberResult](domain.NewDomainError(domain.ErrCodeWORKSPACE_NOT_FOUND, "workspace not found", nil, domain.SeverityMEDIUM, false))
		}
		return domain.Fail[InviteMemberResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// ③ Vérifier que l'acteur est membre du workspace
	actor, err := uc.workspaceRepo.GetMember(ctx, cmd.WorkspaceID, cmd.ActorID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Fail[InviteMemberResult](domain.NewDomainError(domain.ErrCodeFORBIDDEN, "actor is not a workspace member", nil, domain.SeverityMEDIUM, false))
		}
		return domain.Fail[InviteMemberResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// WS-06 : seuls Owner, Admin, Manager peuvent inviter
	if !actor.Role.IsAtLeast(domain.MemberRoleManager) {
		return domain.Fail[InviteMemberResult](domain.NewDomainError(domain.ErrCodeFORBIDDEN, "insufficient role to invite members", nil, domain.SeverityMEDIUM, false))
	}

	// WS-06 note ¹ : ne pas inviter un rôle supérieur au sien
	invitedRole := domain.MemberRole(cmd.Role)
	if !actor.Role.IsAtLeast(invitedRole) {
		return domain.Fail[InviteMemberResult](domain.NewDomainError(domain.ErrCodeCANNOT_INVITE_HIGHER_ROLE, "cannot invite a member with a higher role than your own", nil, domain.SeverityMEDIUM, false))
	}

	// U-4 : email de l'acteur vérifié
	actorUser, err := uc.userRepo.GetByID(ctx, cmd.ActorID)
	if err != nil {
		return domain.Fail[InviteMemberResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}
	if !actorUser.EmailVerified() {
		return domain.Fail[InviteMemberResult](domain.NewDomainError(domain.ErrCodeEMAIL_NOT_VERIFIED, "actor email must be verified to invite members", nil, domain.SeverityMEDIUM, false))
	}

	// M-5 : limite de membres du plan
	if err := uc.planChecker.CheckMemberLimit(ctx, cmd.WorkspaceID); err != nil {
		return domain.Fail[InviteMemberResult](domain.NewDomainError(domain.ErrCodeMEMBER_LIMIT_REACHED, err.Error(), nil, domain.SeverityMEDIUM, false))
	}

	// Vérifier absence d'invitation pending pour cet email
	existingInvitations, err := uc.invitationRepo.ListByWorkspace(ctx, cmd.WorkspaceID)
	if err != nil {
		return domain.Fail[InviteMemberResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}
	for _, inv := range existingInvitations {
		if strings.EqualFold(inv.Email, cmd.Email) && inv.Status == domain.InvitationStatusPending {
			return domain.Fail[InviteMemberResult](domain.NewDomainError(domain.ErrCodeINVITATION_ALREADY_PENDING, "a pending invitation already exists for this email", nil, domain.SeverityMEDIUM, false))
		}
	}

	// Vérifier que l'utilisateur n'est pas déjà membre (si son compte existe)
	existingUser, lookupErr := uc.userRepo.GetByEmail(ctx, cmd.Email)
	if lookupErr == nil && existingUser != nil {
		_, memberErr := uc.workspaceRepo.GetMember(ctx, cmd.WorkspaceID, existingUser.ID())
		if memberErr == nil {
			return domain.Fail[InviteMemberResult](domain.NewDomainError(domain.ErrCodeALREADY_MEMBER, "user is already a workspace member", nil, domain.SeverityMEDIUM, false))
		}
	}

	// ④ Générer le token (I-4 : 64 octets aléatoires, hex-encodé = 128 chars)
	token, err := generateInvitationToken()
	if err != nil {
		return domain.Fail[InviteMemberResult](domain.NewDomainError(domain.ErrCodeINTERNAL, "failed to generate invitation token", nil, domain.SeverityHIGH, true))
	}

	// I-1 : TTL = 7 jours
	now := time.Now().UTC()
	expiresAt := now.Add(7 * 24 * time.Hour)

	invitation := &domain.WorkspaceInvitation{
		ID:              uuid.New(),
		WorkspaceID:     cmd.WorkspaceID,
		InviterMemberID: actor.ID,
		Email:           strings.ToLower(cmd.Email),
		Role:            invitedRole,
		Token:           token,
		Status:          domain.InvitationStatusPending,
		ExpiresAt:       expiresAt,
		CreatedAt:       now,
	}

	// ⑤ Persister
	if err := uc.invitationRepo.Create(ctx, invitation); err != nil {
		return domain.Fail[InviteMemberResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// ⑥ Émettre l'événement
	_ = uc.eventBus.Publish(ctx, events.NewWorkspaceMemberInvitedEvent(
		cmd.WorkspaceID,       // workspaceID uuid.UUID
		invitation.Email,      // email string
		string(invitedRole),   // role string
		cmd.ActorID.String(),  // invitedByUserID string
		expiresAt,             // expiresAt time.Time
	))

	// ⑦ DTO
	return domain.Ok(InviteMemberResult{
		InvitationID: invitation.ID.String(),
		Email:        invitation.Email,
		Role:         string(invitedRole),
		ExpiresAt:    expiresAt,
	})
}

func generateInvitationToken() (string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ─────────────────────────────────────────────────────────
// AcceptInvitationUseCase
// ─────────────────────────────────────────────────────────

type AcceptInvitationCommand struct {
	ActorID uuid.UUID `validate:"required"`
	Token   string    `validate:"required,len=128"`
}

type AcceptInvitationResult struct {
	MemberID    string    `json:"member_id"`
	WorkspaceID string    `json:"workspace_id"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}

type AcceptInvitationUseCase struct {
	workspaceRepo  domain.IWorkspaceRepo
	invitationRepo domain.IInvitationRepo
	userRepo       domain.IUserRepo
	eventBus       domain.IEventBus
	planChecker    PlanLimitsChecker
}

func NewAcceptInvitationUseCase(
	workspaceRepo domain.IWorkspaceRepo,
	invitationRepo domain.IInvitationRepo,
	userRepo domain.IUserRepo,
	eventBus domain.IEventBus,
	planChecker PlanLimitsChecker,
) *AcceptInvitationUseCase {
	return &AcceptInvitationUseCase{
		workspaceRepo:  workspaceRepo,
		invitationRepo: invitationRepo,
		userRepo:       userRepo,
		eventBus:       eventBus,
		planChecker:    planChecker,
	}
}

func (uc *AcceptInvitationUseCase) Execute(ctx context.Context, cmd AcceptInvitationCommand) domain.Result[AcceptInvitationResult] {
	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[AcceptInvitationResult](domain.NewDomainError(domain.ErrCodeVALIDATION, err.Error(), nil, domain.SeverityLOW, false))
	}

	// ② Charger l'invitation par token (I-3)
	invitation, err := uc.invitationRepo.GetByToken(ctx, cmd.Token)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Fail[AcceptInvitationResult](domain.NewDomainError(domain.ErrCodeINVITATION_NOT_FOUND, "invitation not found", nil, domain.SeverityMEDIUM, false))
		}
		return domain.Fail[AcceptInvitationResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// SM-05 : état pending requis (accepted/cancelled/expired = terminaux)
	if invitation.Status != domain.InvitationStatusPending {
		return domain.Fail[AcceptInvitationResult](domain.NewDomainError(domain.ErrCodeINVITATION_NOT_PENDING, "invitation is no longer pending", nil, domain.SeverityMEDIUM, false))
	}

	// Expiration : transition pending → expired si TTL dépassé
	if time.Now().UTC().After(invitation.ExpiresAt) {
		invitation.Status = domain.InvitationStatusExpired
		_ = uc.invitationRepo.Update(ctx, invitation)
		return domain.Fail[AcceptInvitationResult](domain.NewDomainError(domain.ErrCodeINVITATION_EXPIRED, "invitation has expired", nil, domain.SeverityMEDIUM, false))
	}

	// ③ Charger l'utilisateur acteur
	actorUser, err := uc.userRepo.GetByID(ctx, cmd.ActorID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Fail[AcceptInvitationResult](domain.NewDomainError(domain.ErrCodeUSER_NOT_FOUND, "user not found", nil, domain.SeverityMEDIUM, false))
		}
		return domain.Fail[AcceptInvitationResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// I-2 : email utilisateur = email invitation
	if !strings.EqualFold(actorUser.Email(), invitation.Email) {
		return domain.Fail[AcceptInvitationResult](domain.NewDomainError(domain.ErrCodeINVITATION_EMAIL_MISMATCH, "your account email does not match the invitation email", nil, domain.SeverityMEDIUM, false))
	}

	// Vérifier que l'utilisateur n'est pas déjà membre
	_, memberErr := uc.workspaceRepo.GetMember(ctx, invitation.WorkspaceID, cmd.ActorID)
	if memberErr == nil {
		return domain.Fail[AcceptInvitationResult](domain.NewDomainError(domain.ErrCodeALREADY_MEMBER, "user is already a workspace member", nil, domain.SeverityMEDIUM, false))
	}

	// M-5 : limite de membres du plan
	if err := uc.planChecker.CheckMemberLimit(ctx, invitation.WorkspaceID); err != nil {
		return domain.Fail[AcceptInvitationResult](domain.NewDomainError(domain.ErrCodeMEMBER_LIMIT_REACHED, err.Error(), nil, domain.SeverityMEDIUM, false))
	}

	// ④ Créer le membre (SM-04 : statut initial = active pour acceptation d'invitation)
	joinedAt := time.Now().UTC()
	memberID := uuid.New()
	member := &domain.WorkspaceMember{
		ID:          memberID,
		WorkspaceID: invitation.WorkspaceID,
		UserID:      cmd.ActorID,
		Role:        invitation.Role,
		Status:      domain.MemberStatusActive,
		JoinedAt:    &joinedAt,
	}

	// ⑤ Persister le membre
	if err := uc.workspaceRepo.AddMember(ctx, member); err != nil {
		return domain.Fail[AcceptInvitationResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// Transition SM-05 : pending → accepted
	invitation.Status = domain.InvitationStatusAccepted
	if err := uc.invitationRepo.Update(ctx, invitation); err != nil {
		return domain.Fail[AcceptInvitationResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// ⑥ Émettre l'événement
	_ = uc.eventBus.Publish(ctx, events.NewWorkspaceMemberAddedEvent(
		invitation.WorkspaceID,
		memberID,
		cmd.ActorID,
		string(invitation.Role),
		joinedAt,
	))

	// ⑦ DTO
	return domain.Ok(AcceptInvitationResult{
		MemberID:    memberID.String(),
		WorkspaceID: invitation.WorkspaceID.String(),
		Role:        string(invitation.Role),
		JoinedAt:    joinedAt,
	})
}
