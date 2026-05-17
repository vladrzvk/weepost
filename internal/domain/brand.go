package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// 1. BrandStatus — Phase 1 P3d §A.6
// ---------------------------------------------------------------------------

type BrandStatus string

const (
	BrandStatusActive          BrandStatus = "active"
	BrandStatusArchived        BrandStatus = "archived"
	BrandStatusPendingDeletion BrandStatus = "pending_deletion"
	BrandStatusDeleted         BrandStatus = "deleted"
)

func (s BrandStatus) IsValid() bool {
	switch s {
	case BrandStatusActive, BrandStatusArchived, BrandStatusPendingDeletion, BrandStatusDeleted:
		return true
	}
	return false
}

func (s BrandStatus) IsOperational() bool { return s == BrandStatusActive }

// ---------------------------------------------------------------------------
// 2. BrandRole — Phase 1 P3d §A.16
// ---------------------------------------------------------------------------

type BrandRole string

const (
	BrandRoleOwner   BrandRole = "owner"
	BrandRoleManager BrandRole = "manager"
	BrandRoleEditor  BrandRole = "editor"
	BrandRoleViewer  BrandRole = "viewer"
)

func (r BrandRole) IsValid() bool {
	switch r {
	case BrandRoleOwner, BrandRoleManager, BrandRoleEditor, BrandRoleViewer:
		return true
	}
	return false
}

func (r BrandRole) CanApprovePost() bool {
	return r == BrandRoleOwner || r == BrandRoleManager
}

// ---------------------------------------------------------------------------
// 3. BrandMemberStatus — Phase 1 P3d §A.16b (calculé, non persisté)
// ---------------------------------------------------------------------------

// BrandMemberStatus est CALCULÉ (non stocké en DB). Dérivé par JOIN sur workspace_members.deleted_at.
type BrandMemberStatus string

const (
	BrandMemberStatusActive   BrandMemberStatus = "active"
	BrandMemberStatusInactive BrandMemberStatus = "inactive"
	BrandMemberStatusPending  BrandMemberStatus = "pending"
)

// ---------------------------------------------------------------------------
// 4. ChannelPermission — Phase 0 §5.1 (niveau 3 granulaire)
// ---------------------------------------------------------------------------

// ChannelPermission représente les permissions granulaires d'un membre sur un channel spécifique.
type ChannelPermission struct {
	ChannelID   uuid.UUID
	Permissions []string // view, create, edit, schedule, publish, moderate, reply, analytics
}

// HasPermission vérifie si une permission est accordée sur ce channel.
func (cp *ChannelPermission) HasPermission(permission string) bool {
	for _, p := range cp.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 5. BrandAssignment — Phase 1 P1 §4.3
// ---------------------------------------------------------------------------

// BrandAssignment lie un WorkspaceMember à une Brand avec un rôle et des permissions channel.
type BrandAssignment struct {
	ID                 uuid.UUID
	WorkspaceID        uuid.UUID
	MemberID           uuid.UUID
	BrandID            uuid.UUID
	Role               BrandRole
	AssignedByMemberID *uuid.UUID
	ChannelPermissions []*ChannelPermission
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// FindChannelPermission retourne les permissions du membre pour un channel donné, ou nil.
func (ba *BrandAssignment) FindChannelPermission(channelID uuid.UUID) *ChannelPermission {
	for _, cp := range ba.ChannelPermissions {
		if cp.ChannelID == channelID {
			return cp
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 6. Brand — aggregate root
// ---------------------------------------------------------------------------

// Brand est un aggregate root. Tous les champs sont privés : accès via getters/méthodes.
type Brand struct {
	id              uuid.UUID
	workspaceID     uuid.UUID
	slug            string
	name            string
	industry        *string
	identity        BrandIdentity
	tone            ToneOfVoice
	status          BrandStatus
	createdByUserID uuid.UUID
	archivedAt      *time.Time
	deletedAt       *time.Time
	createdAt       time.Time
	updatedAt       time.Time
	assignments     []*BrandAssignment
}

// Getters

func (b *Brand) ID() uuid.UUID                    { return b.id }
func (b *Brand) WorkspaceID() uuid.UUID           { return b.workspaceID }
func (b *Brand) Slug() string                     { return b.slug }
func (b *Brand) Name() string                     { return b.name }
func (b *Brand) Industry() *string                { return b.industry }
func (b *Brand) Identity() BrandIdentity          { return b.identity }
func (b *Brand) Tone() ToneOfVoice                { return b.tone }
func (b *Brand) Status() BrandStatus              { return b.status }
func (b *Brand) CreatedByUserID() uuid.UUID       { return b.createdByUserID }
func (b *Brand) ArchivedAt() *time.Time           { return b.archivedAt }
func (b *Brand) DeletedAt() *time.Time            { return b.deletedAt }
func (b *Brand) CreatedAt() time.Time             { return b.createdAt }
func (b *Brand) UpdatedAt() time.Time             { return b.updatedAt }
func (b *Brand) Assignments() []*BrandAssignment  { return b.assignments }

// NewBrand crée une nouvelle brand. Invariants B-1 (nom 2-100 chars).
func NewBrand(workspaceID, createdByUserID uuid.UUID, name string) (*Brand, error) {
	trimmedName := strings.TrimSpace(name)
	if len([]rune(trimmedName)) < 2 {
		return nil, NewDomainError(
			ErrCodeINVALID_BRAND_NAME,
			"Le nom de la brand doit contenir au moins 2 caractères",
			map[string]interface{}{"field": "name", "min_length": 2},
			SeverityLOW,
			false,
		)
	}
	if len([]rune(trimmedName)) > 100 {
		return nil, NewDomainError(
			ErrCodeBRAND_NAME_TOO_LONG,
			"Le nom de la brand ne peut pas dépasser 100 caractères",
			map[string]interface{}{"field": "name", "max_length": 100},
			SeverityLOW,
			false,
		)
	}
	now := time.Now().UTC()
	b := &Brand{
		id:              uuid.New(),
		workspaceID:     workspaceID,
		name:            trimmedName,
		status:          BrandStatusActive,
		createdByUserID: createdByUserID,
		createdAt:       now,
		updatedAt:       now,
	}
	b.slug = generateSlug(trimmedName, b.id)
	return b, nil
}

// Archive transite active→archived. Invariant B-2 (brand active uniquement).
func (b *Brand) Archive(actorID uuid.UUID) error {
	if b.status == BrandStatusArchived {
		return NewDomainError(
			ErrCodeBRAND_ALREADY_ARCHIVED,
			"La brand est déjà archivée",
			map[string]interface{}{"brand_id": b.id.String()},
			SeverityLOW,
			false,
		)
	}
	if b.status != BrandStatusActive {
		return NewDomainError(
			ErrCodeINVALID_STATUS_TRANSITION,
			"Seule une brand active peut être archivée",
			map[string]interface{}{
				"brand_id":       b.id.String(),
				"current_status": string(b.status),
			},
			SeverityMEDIUM,
			false,
		)
	}
	now := time.Now().UTC()
	b.status = BrandStatusArchived
	b.archivedAt = &now
	b.updatedAt = now
	return nil
}

// Unarchive restaure une brand archivée vers l'état actif.
// Transition state machine : archived → active.
func (b *Brand) Unarchive() error {
	if b.status != BrandStatusArchived {
		return NewDomainError(
			ErrCodeINVALID_STATUS_TRANSITION,
			"Seule une brand archivée peut être désarchivée",
			map[string]interface{}{
				"brand_id":       b.id.String(),
				"current_status": string(b.status),
				"target_status":  string(BrandStatusActive),
			},
			SeverityMEDIUM,
			false,
		)
	}
	b.status = BrandStatusActive
	b.updatedAt = time.Now().UTC()
	return nil
}

// CanCreatePost vérifie l'invariant B-2 : seule une brand active peut créer des posts.
func (b *Brand) CanCreatePost() bool {
	return b.status == BrandStatusActive
}

// AssignMember affecte un membre à la brand avec un rôle donné.
// Invariant B-5 : le bypass Owner est géré au niveau PermissionChecker (use case layer).
// Cette méthode vérifie uniquement l'état brand + unicité de l'affectation.
func (b *Brand) AssignMember(memberID uuid.UUID, role BrandRole, actorID uuid.UUID) error {
	if !b.status.IsOperational() {
		return NewDomainError(
			ErrCodeBRAND_ACCESS_DENIED,
			"Impossible d'affecter un membre à une brand non opérationnelle",
			map[string]interface{}{
				"brand_id": b.id.String(),
				"status":   string(b.status),
			},
			SeverityLOW,
			false,
		)
	}
	// Vérification doublon : un membre ne peut être affecté qu'une fois à une brand
	for _, a := range b.assignments {
		if a.MemberID == memberID {
			return NewDomainError(
				ErrCodeMEMBER_ALREADY_EXISTS,
				"Ce membre est déjà affecté à cette brand",
				map[string]interface{}{
					"brand_id":  b.id.String(),
					"member_id": memberID.String(),
				},
				SeverityLOW,
				false,
			)
		}
	}
	if !role.IsValid() {
		return NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le rôle brand fourni n'est pas valide",
			map[string]interface{}{"role": string(role)},
			SeverityLOW,
			false,
		)
	}
	now := time.Now().UTC()
	assignment := &BrandAssignment{
		ID:                 uuid.New(),
		WorkspaceID:        b.workspaceID,
		MemberID:           memberID,
		BrandID:            b.id,
		Role:               role,
		AssignedByMemberID: &actorID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	b.assignments = append(b.assignments, assignment)
	b.updatedAt = now
	return nil
}

// RemoveMember retire l'affectation d'un membre à la brand.
func (b *Brand) RemoveMember(memberID uuid.UUID, actorID uuid.UUID) error {
	for i, a := range b.assignments {
		if a.MemberID == memberID {
			b.assignments = append(b.assignments[:i], b.assignments[i+1:]...)
			b.updatedAt = time.Now().UTC()
			return nil
		}
	}
	return NewDomainError(
		ErrCodeNO_ASSIGNMENT_TO_BRAND,
		"Ce membre n'est pas affecté à cette brand",
		map[string]interface{}{
			"brand_id":  b.id.String(),
			"member_id": memberID.String(),
		},
		SeverityLOW,
		false,
	)
}

// GetAssignmentByMemberID retourne l'affectation du membre, ou nil.
func (b *Brand) GetAssignmentByMemberID(memberID uuid.UUID) *BrandAssignment {
	for _, a := range b.assignments {
		if a.MemberID == memberID {
			return a
		}
	}
	return nil
}

// Rename modifie le nom de la brand.
// L'unicité du nom dans le workspace (invariant B-1) doit être vérifiée
// par le Use Case appelant via IBrandRepo.ExistsBySlugInWorkspace
// avant d'invoquer cette méthode.
func (b *Brand) Rename(newName string) error {
	if len(newName) < 2 || len(newName) > 100 {
		return NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le nom de la brand doit contenir entre 2 et 100 caractères",
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
	if b.status == BrandStatusDeleted {
		return NewDomainError(
			ErrCodeINVALID_STATUS_TRANSITION,
			"Impossible de renommer une brand supprimée",
			map[string]interface{}{
				"brand_id":       b.id.String(),
				"current_status": string(b.status),
			},
			SeverityMEDIUM,
			false,
		)
	}
	b.name = newName
	b.slug = generateSlug(strings.TrimSpace(newName), b.id)
	b.updatedAt = time.Now().UTC()
	return nil
}
