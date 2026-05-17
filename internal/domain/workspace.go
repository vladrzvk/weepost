package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// 1. WorkspaceStatus — Phase 1 P3d §A.2 — SM-02
// ---------------------------------------------------------------------------

type WorkspaceStatus string

const (
	WorkspaceStatusActive          WorkspaceStatus = "active"
	WorkspaceStatusSuspended       WorkspaceStatus = "suspended"
	WorkspaceStatusPendingDeletion WorkspaceStatus = "pending_deletion"
	WorkspaceStatusDeleted         WorkspaceStatus = "deleted"
)

var workspaceStatusTransitions = map[WorkspaceStatus][]WorkspaceStatus{
	WorkspaceStatusActive:          {WorkspaceStatusSuspended, WorkspaceStatusPendingDeletion},
	WorkspaceStatusSuspended:       {WorkspaceStatusActive, WorkspaceStatusPendingDeletion},
	WorkspaceStatusPendingDeletion: {WorkspaceStatusActive, WorkspaceStatusDeleted},
	WorkspaceStatusDeleted:         {},
}

func (s WorkspaceStatus) IsValid() bool {
	switch s {
	case WorkspaceStatusActive, WorkspaceStatusSuspended,
		WorkspaceStatusPendingDeletion, WorkspaceStatusDeleted:
		return true
	}
	return false
}

func (s WorkspaceStatus) CanTransitionTo(target WorkspaceStatus) bool {
	allowed := workspaceStatusTransitions[s]
	for _, t := range allowed {
		if t == target {
			return true
		}
	}
	return false
}

func (s WorkspaceStatus) IsTerminal() bool { return s == WorkspaceStatusDeleted }

// ---------------------------------------------------------------------------
// 2. WorkspaceMode — Phase 1 P3d §A.1 — W-3
// ---------------------------------------------------------------------------

type WorkspaceMode string

const (
	WorkspaceModeSimple WorkspaceMode = "simple"
	WorkspaceModeTeam   WorkspaceMode = "team"
	WorkspaceModeAgency WorkspaceMode = "agency"
)

func (m WorkspaceMode) IsValid() bool {
	switch m {
	case WorkspaceModeSimple, WorkspaceModeTeam, WorkspaceModeAgency:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// 3. MemberRole — Phase 1 P3d §A.3
// ---------------------------------------------------------------------------

type MemberRole string

const (
	MemberRoleOwner   MemberRole = "owner"
	MemberRoleAdmin   MemberRole = "admin"
	MemberRoleManager MemberRole = "manager"
	MemberRoleEditor  MemberRole = "editor"
	MemberRoleViewer  MemberRole = "viewer"
)

func (r MemberRole) IsValid() bool {
	switch r {
	case MemberRoleOwner, MemberRoleAdmin, MemberRoleManager, MemberRoleEditor, MemberRoleViewer:
		return true
	}
	return false
}

func (r MemberRole) Level() int {
	levels := map[MemberRole]int{
		MemberRoleOwner:   5,
		MemberRoleAdmin:   4,
		MemberRoleManager: 3,
		MemberRoleEditor:  2,
		MemberRoleViewer:  1,
	}
	return levels[r]
}

func (r MemberRole) IsAtLeast(minimum MemberRole) bool {
	return r.Level() >= minimum.Level()
}

// BypassesBrandAssignment — seul Owner bypasse B-5 (A003 — pas Admin).
func (r MemberRole) BypassesBrandAssignment() bool {
	return r == MemberRoleOwner
}

// ---------------------------------------------------------------------------
// 4. MemberStatus — Phase 1 P3d §A.4
// ---------------------------------------------------------------------------

type MemberStatus string

const (
	MemberStatusActive    MemberStatus = "active"
	MemberStatusInactive  MemberStatus = "inactive"
	MemberStatusPending   MemberStatus = "pending"
	MemberStatusInvited   MemberStatus = "invited" // ANOMALIE A011 : absent de la liste canonique Phase 5 SM-04 — maintenu pour compatibilité
	MemberStatusSuspended MemberStatus = "suspended"
)

func (s MemberStatus) IsValid() bool {
	switch s {
	case MemberStatusActive, MemberStatusInactive, MemberStatusPending,
		MemberStatusInvited, MemberStatusSuspended:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// 5. WorkspaceMember — Phase 1 P1 §3.1.2
// ---------------------------------------------------------------------------

// WorkspaceMember est une entité enfant de Workspace (pas un aggregate root).
type WorkspaceMember struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	UserID      uuid.UUID
	Role        MemberRole
	Status      MemberStatus
	InvitedByMemberID *uuid.UUID
	JoinedAt    *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// IsActive retourne true si le membre est actif et non supprimé.
func (m *WorkspaceMember) IsActive() bool {
	return m.Status == MemberStatusActive && m.DeletedAt == nil
}

// ---------------------------------------------------------------------------
// 6. Workspace — aggregate root
// ---------------------------------------------------------------------------

// Workspace est l'aggregate root. Tous les champs sont privés : accès via getters/méthodes.
type Workspace struct {
	id                 uuid.UUID
	slug               string
	name               string
	description        string
	ownerUserID        uuid.UUID
	mode               WorkspaceMode
	planID             string
	status             WorkspaceStatus
	settings           *WorkspaceSettings
	limits             WorkspaceLimits
	brandCount         int
	guestPortalEnabled bool
	createdAt          time.Time
	updatedAt          time.Time
	deletedAt          *time.Time
	members            []*WorkspaceMember
	events             []DomainEvent
}

// Getters

func (w *Workspace) ID() uuid.UUID                { return w.id }
func (w *Workspace) Slug() string                 { return w.slug }
func (w *Workspace) Name() string                 { return w.name }
func (w *Workspace) Description() string          { return w.description }
func (w *Workspace) OwnerUserID() uuid.UUID       { return w.ownerUserID }
func (w *Workspace) Mode() WorkspaceMode          { return w.mode }
func (w *Workspace) PlanID() string                { return w.planID }
func (w *Workspace) Status() WorkspaceStatus      { return w.status }
func (w *Workspace) Settings() *WorkspaceSettings { return w.settings }
func (w *Workspace) Limits() WorkspaceLimits      { return w.limits }
func (w *Workspace) BrandCount() int              { return w.brandCount }
func (w *Workspace) GuestPortalEnabled() bool     { return w.guestPortalEnabled }
func (w *Workspace) CreatedAt() time.Time         { return w.createdAt }
func (w *Workspace) UpdatedAt() time.Time         { return w.updatedAt }
func (w *Workspace) DeletedAt() *time.Time        { return w.deletedAt }
func (w *Workspace) Members() []*WorkspaceMember  { return w.members }
func (w *Workspace) Events() []DomainEvent        { return w.events }

// ClearEvents vide la liste des événements après dispatch.
func (w *Workspace) ClearEvents() { w.events = nil }

// NewWorkspace crée un workspace avec slug auto-généré et membre Owner auto-créé.
// Invariants : W-1 (exactement 1 Owner), W-3 (mode auto-calculé).
func NewWorkspace(ownerID uuid.UUID, name, timezone, language string) (*Workspace, error) {
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) < 2 || len(trimmedName) > 100 {
		return nil, NewDomainError(
			ErrCodeINVALID_WORKSPACE_NAME,
			"Le nom du workspace doit contenir entre 2 et 100 caractères",
			map[string]interface{}{
				"field":      "name",
				"min_length": 2,
				"max_length": 100,
			},
			SeverityLOW,
			false,
		)
	}
	settings, err := NewWorkspaceSettings(timezone, language, "DD/MM/YYYY", "24h", 1)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	ownerMember := &WorkspaceMember{
		ID:          uuid.New(),
		WorkspaceID: uuid.Nil, // sera mis à jour après création
		UserID:      ownerID,
		Role:        MemberRoleOwner,
		Status:      MemberStatusActive,
		JoinedAt:    &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	ws := &Workspace{
		id:          uuid.New(),
		name:        trimmedName,
		ownerUserID: ownerID,
		planID:      "free",
		mode:        WorkspaceModeSimple,
		status:      WorkspaceStatusActive,
		settings:    settings,
		limits:      WorkspaceLimits{MaxMembers: 10, MaxBrands: 3, MaxChannels: 10},
		brandCount:  0,
		createdAt:   now,
		updatedAt:   now,
		members:     []*WorkspaceMember{ownerMember},
	}
	ws.slug = generateSlug(trimmedName, ws.id)
	ownerMember.WorkspaceID = ws.id
	return ws, nil
}

// Suspend transite active→suspended. Acteur doit être Owner (W-5).
func (w *Workspace) Suspend(actorID uuid.UUID) error {
	if err := w.validateActorIsOwner(actorID); err != nil {
		return err
	}
	if !w.status.CanTransitionTo(WorkspaceStatusSuspended) {
		return NewDomainError(
			ErrCodeINVALID_STATUS_TRANSITION,
			"Impossible de suspendre le workspace depuis l'état actuel",
			map[string]interface{}{
				"current_status": string(w.status),
				"target_status":  string(WorkspaceStatusSuspended),
			},
			SeverityMEDIUM,
			false,
		)
	}
	now := time.Now().UTC()
	w.status = WorkspaceStatusSuspended
	w.updatedAt = now
	return nil
}

// Delete transite vers pending_deletion. Acteur doit être Owner (W-5).
func (w *Workspace) Delete(actorID uuid.UUID) error {
	if err := w.validateActorIsOwner(actorID); err != nil {
		return err
	}
	if !w.status.CanTransitionTo(WorkspaceStatusPendingDeletion) {
		if w.status == WorkspaceStatusDeleted {
			return NewDomainError(
				ErrCodeWORKSPACE_ALREADY_DELETED,
				"Le workspace est déjà supprimé",
				map[string]interface{}{"workspace_id": w.id.String()},
				SeverityLOW,
				false,
			)
		}
		return NewDomainError(
			ErrCodeINVALID_STATUS_TRANSITION,
			"Impossible de supprimer le workspace depuis l'état actuel",
			map[string]interface{}{
				"current_status": string(w.status),
				"target_status":  string(WorkspaceStatusPendingDeletion),
			},
			SeverityMEDIUM,
			false,
		)
	}
	now := time.Now().UTC()
	w.status = WorkspaceStatusPendingDeletion
	w.deletedAt = &now
	w.updatedAt = now
	return nil
}

// UpdateSettings remplace les settings. Workspace doit être actif.
func (w *Workspace) UpdateSettings(settings *WorkspaceSettings, actorID uuid.UUID) error {
	if w.status != WorkspaceStatusActive {
		return NewDomainError(
			ErrCodeWORKSPACE_SUSPENDED,
			"Impossible de modifier les paramètres d'un workspace inactif",
			map[string]interface{}{
				"workspace_id": w.id.String(),
				"status":       string(w.status),
			},
			SeverityLOW,
			false,
		)
	}
	if settings == nil {
		return NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Les paramètres ne peuvent pas être nuls",
			map[string]interface{}{"field": "settings"},
			SeverityLOW,
			false,
		)
	}
	w.settings = settings
	w.updatedAt = time.Now().UTC()
	return nil
}

// AddMember ajoute un membre au workspace. Invariant M-2 (pas de doublon).
func (w *Workspace) AddMember(userID uuid.UUID, role MemberRole, invitedByID uuid.UUID) (*WorkspaceMember, error) {
	if w.status != WorkspaceStatusActive {
		return nil, NewDomainError(
			ErrCodeWORKSPACE_SUSPENDED,
			"Impossible d'ajouter un membre à un workspace inactif",
			map[string]interface{}{"workspace_id": w.id.String()},
			SeverityLOW,
			false,
		)
	}
	// M-2 : pas de doublon utilisateur actif
	for _, m := range w.members {
		if m.UserID == userID && m.DeletedAt == nil {
			return nil, NewDomainError(
				ErrCodeMEMBER_ALREADY_EXISTS,
				"Cet utilisateur est déjà membre du workspace",
				map[string]interface{}{
					"workspace_id": w.id.String(),
					"user_id":      userID.String(),
				},
				SeverityLOW,
				false,
			)
		}
	}
	// Rôle Owner ne peut pas être assigné via AddMember (W-1)
	if role == MemberRoleOwner {
		return nil, NewDomainError(
			ErrCodeWORKSPACE_MUST_HAVE_OWNER,
			"Le rôle Owner ne peut pas être assigné via cette opération",
			map[string]interface{}{"workspace_id": w.id.String()},
			SeverityHIGH,
			false,
		)
	}
	now := time.Now().UTC()
	member := &WorkspaceMember{
		ID:          uuid.New(),
		WorkspaceID: w.id,
		UserID:      userID,
		Role:        role,
		Status:      MemberStatusInvited,
		InvitedByMemberID: &invitedByID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	w.members = append(w.members, member)
	w.updatedAt = now
	w.recomputeMode()
	return member, nil
}

// RemoveMember retire un membre. Invariant W-1 (Owner ne peut pas être retiré si seul Owner).
// Invariant M-4 : acteur doit être Owner ou Admin, ou le membre lui-même.
func (w *Workspace) RemoveMember(memberUserID uuid.UUID, actorID uuid.UUID) error {
	var targetMember *WorkspaceMember
	for _, m := range w.members {
		if m.UserID == memberUserID && m.DeletedAt == nil {
			targetMember = m
			break
		}
	}
	if targetMember == nil {
		return NewDomainError(
			ErrCodeMEMBER_NOT_FOUND,
			"Membre non trouvé dans le workspace",
			map[string]interface{}{
				"workspace_id": w.id.String(),
				"user_id":      memberUserID.String(),
			},
			SeverityLOW,
			false,
		)
	}
	// W-1 : Owner ne peut être retiré que si un autre Owner existe
	if targetMember.Role == MemberRoleOwner {
		if err := w.validateOwnerCount(); err != nil {
			return NewDomainError(
				ErrCodeCANNOT_REMOVE_OWNER,
				"Impossible de retirer l'unique Owner du workspace",
				map[string]interface{}{"workspace_id": w.id.String()},
				SeverityHIGH,
				false,
			)
		}
	}
	now := time.Now().UTC()
	targetMember.DeletedAt = &now
	targetMember.UpdatedAt = now
	w.updatedAt = now
	w.recomputeMode()
	return nil
}

// SetDescription modifie la description du workspace.
func (w *Workspace) SetDescription(desc string) {
	w.description = desc
	w.updatedAt = time.Now().UTC()
}

// Rename modifie le nom du workspace.
// L'unicité du nom (invariant W-1) doit être vérifiée par le Use Case
// appelant via IWorkspaceRepo.ExistsByName avant d'invoquer cette méthode.
func (w *Workspace) Rename(newName string) error {
	if len(newName) < 2 || len(newName) > 100 {
		return NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le nom du workspace doit contenir entre 2 et 100 caractères",
			map[string]interface{}{
				"field":      "name",
				"length":     len(newName),
				"min_length": 2,
				"max_length": 100,
			},
			SeverityLOW,
			false,
		)
	}
	if w.status == WorkspaceStatusDeleted {
		return NewDomainError(
			ErrCodeINVALID_STATUS_TRANSITION,
			"Impossible de renommer un workspace supprimé",
			map[string]interface{}{
				"workspace_id":   w.id.String(),
				"current_status": string(w.status),
			},
			SeverityMEDIUM,
			false,
		)
	}
	w.name = newName
	w.updatedAt = time.Now().UTC()
	return nil
}

// GetActiveMemberCount retourne le nombre de membres actifs non supprimés.
func (w *Workspace) GetActiveMemberCount() int {
	count := 0
	for _, m := range w.members {
		if m.IsActive() {
			count++
		}
	}
	return count
}

// GetMemberByUserID retourne le membre actif correspondant à userID, ou nil.
func (w *Workspace) GetMemberByUserID(userID uuid.UUID) *WorkspaceMember {
	for _, m := range w.members {
		if m.UserID == userID && m.DeletedAt == nil {
			return m
		}
	}
	return nil
}

// validateActorIsOwner vérifie que l'acteur est bien Owner du workspace.
func (w *Workspace) validateActorIsOwner(actorID uuid.UUID) error {
	member := w.GetMemberByUserID(actorID)
	if member == nil || member.Role != MemberRoleOwner {
		return NewDomainError(
			ErrCodeNOT_WORKSPACE_OWNER,
			"Seul l'Owner peut effectuer cette opération",
			map[string]interface{}{
				"workspace_id": w.id.String(),
				"actor_id":     actorID.String(),
			},
			SeverityMEDIUM,
			false,
		)
	}
	return nil
}

// validateOwnerCount vérifie qu'il reste au moins 2 Owners avant retrait (W-1).
func (w *Workspace) validateOwnerCount() error {
	ownerCount := 0
	for _, m := range w.members {
		if m.Role == MemberRoleOwner && m.DeletedAt == nil {
			ownerCount++
		}
	}
	if ownerCount <= 1 {
		return NewDomainError(
			ErrCodeWORKSPACE_MUST_HAVE_OWNER,
			"Le workspace doit toujours avoir au moins un Owner",
			map[string]interface{}{"workspace_id": w.id.String()},
			SeverityHIGH,
			false,
		)
	}
	return nil
}

// recomputeMode recalcule le mode workspace selon W-3.
// simple→team si members>1 ou brands>1 ; team→agency si brands≥5 ou guestPortal activé.
func (w *Workspace) recomputeMode() {
	activeMembers := w.GetActiveMemberCount()
	if w.brandCount >= 5 || w.guestPortalEnabled {
		w.mode = WorkspaceModeAgency
		return
	}
	if activeMembers > 1 || w.brandCount > 1 {
		w.mode = WorkspaceModeTeam
		return
	}
	w.mode = WorkspaceModeSimple
}

var nonAlphanumRegex = regexp.MustCompile(`[^a-z0-9]+`)

// generateSlug crée un slug kebab-case depuis le nom, suffixé par les 8 premiers chars de l'UUID.
// L'unicité est garantie au niveau use case (vérification en base).
func generateSlug(name string, id uuid.UUID) string {
	lower := strings.ToLower(name)
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '-'
	}, lower)
	slug := nonAlphanumRegex.ReplaceAllString(normalized, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "workspace"
	}
	suffix := strings.ReplaceAll(id.String(), "-", "")[:8]
	return slug + "-" + suffix
}

