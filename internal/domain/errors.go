package domain

import (
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// 1. ErrorCode — type + constantes exhaustives (Phase 0 §4)
// ---------------------------------------------------------------------------

// ErrorCode identifie de manière unique un type d'erreur.
// Format : {CATEGORY}_{DESCRIPTION}
type ErrorCode string

const (
	// ── Authentication ──────────────────────────────────────────────────────
	ErrCodeAUTHENTICATION_FAILED ErrorCode = "AUTHENTICATION_FAILED"
	ErrCodeINVALID_CREDENTIALS   ErrorCode = "INVALID_CREDENTIALS"
	ErrCodeACCOUNT_LOCKED        ErrorCode = "ACCOUNT_LOCKED"
	ErrCodeTOO_MANY_ATTEMPTS     ErrorCode = "TOO_MANY_ATTEMPTS"
	ErrCode2FA_REQUIRED          ErrorCode = "2FA_REQUIRED"
	ErrCodeINVALID_2FA_CODE      ErrorCode = "INVALID_2FA_CODE"
	ErrCodeEMAIL_NOT_VERIFIED    ErrorCode = "EMAIL_NOT_VERIFIED"
	ErrCodeINVALID_TOKEN         ErrorCode = "INVALID_TOKEN"

	// ── Workspace ────────────────────────────────────────────────────────────
	ErrCodeINVALID_WORKSPACE_NAME    ErrorCode = "INVALID_WORKSPACE_NAME"
	ErrCodeSLUG_ALREADY_EXISTS       ErrorCode = "SLUG_ALREADY_EXISTS"
	ErrCodeSLUG_INVALID_FORMAT       ErrorCode = "SLUG_INVALID_FORMAT"
	ErrCodeWORKSPACE_CREATION_FAILED ErrorCode = "WORKSPACE_CREATION_FAILED"
	ErrCodeWORKSPACE_SUSPENDED       ErrorCode = "WORKSPACE_SUSPENDED"
	ErrCodeDUPLICATE_FREE_WORKSPACE  ErrorCode = "DUPLICATE_FREE_WORKSPACE"
	ErrCodeWORKSPACE_NOT_FOUND       ErrorCode = "WORKSPACE_NOT_FOUND"
	ErrCodeWORKSPACE_ALREADY_DELETED ErrorCode = "WORKSPACE_ALREADY_DELETED"
	ErrCodeCANNOT_DELETE_ACTIVE_WORKSPACE ErrorCode = "CANNOT_DELETE_ACTIVE_WORKSPACE"
	// WORKSPACE_MUST_HAVE_ONE_OWNER : valeur string intentionnellement différente du nom de constante
	ErrCodeWORKSPACE_MUST_HAVE_OWNER ErrorCode = "WORKSPACE_MUST_HAVE_ONE_OWNER"
	ErrCodeWORKSPACE_ACCESS_DENIED   ErrorCode = "WORKSPACE_ACCESS_DENIED"
	ErrCodeNOT_WORKSPACE_OWNER       ErrorCode = "NOT_WORKSPACE_OWNER"
	ErrCodeMEMBER_NOT_FOUND          ErrorCode = "MEMBER_NOT_FOUND"
	ErrCodeCANNOT_REMOVE_OWNER       ErrorCode = "CANNOT_REMOVE_OWNER"
	ErrCodeMEMBER_ALREADY_EXISTS     ErrorCode = "MEMBER_ALREADY_EXISTS"
	ErrCodeMEMBER_CREATION_FAILED    ErrorCode = "MEMBER_CREATION_FAILED"

	// ── Brand ────────────────────────────────────────────────────────────────
	ErrCodeINVALID_BRAND_NAME             ErrorCode = "INVALID_BRAND_NAME"
	ErrCodeBRAND_NAME_TOO_LONG            ErrorCode = "BRAND_NAME_TOO_LONG"
	ErrCodeBRAND_NOT_FOUND                ErrorCode = "BRAND_NOT_FOUND"
	ErrCodeBRAND_ALREADY_ARCHIVED         ErrorCode = "BRAND_ALREADY_ARCHIVED"
	ErrCodeBRAND_ARCHIVED                 ErrorCode = "BRAND_ARCHIVED"
	ErrCodeBRAND_ACCESS_DENIED            ErrorCode = "BRAND_ACCESS_DENIED"
	ErrCodeNO_ASSIGNMENT_TO_BRAND         ErrorCode = "NO_ASSIGNMENT_TO_BRAND"
	ErrCodeNOT_BRAND_OWNER                ErrorCode = "NOT_BRAND_OWNER"
	ErrCodeBRAND_ASSIGNMENT_ALREADY_EXISTS ErrorCode = "BRAND_ASSIGNMENT_ALREADY_EXISTS"
	ErrCodeBRAND_ASSIGNMENT_NOT_FOUND     ErrorCode = "BRAND_ASSIGNMENT_NOT_FOUND"

	// ── Channel ──────────────────────────────────────────────────────────────
	ErrCodeCHANNEL_NOT_FOUND         ErrorCode = "CHANNEL_NOT_FOUND"
	ErrCodeCHANNEL_ALREADY_CONNECTED ErrorCode = "CHANNEL_ALREADY_CONNECTED"
	ErrCodeCHANNEL_REVOKED           ErrorCode = "CHANNEL_REVOKED"
	ErrCodeTOKEN_EXPIRED             ErrorCode = "TOKEN_EXPIRED"
	ErrCodeINVALID_OAUTH_TOKEN       ErrorCode = "INVALID_OAUTH_TOKEN"
	ErrCodeCHANNEL_ACCESS_DENIED     ErrorCode = "CHANNEL_ACCESS_DENIED"
	ErrCodeCHANNEL_PUBLISH_DENIED    ErrorCode = "CHANNEL_PUBLISH_DENIED"
	ErrCodeCHANNEL_DISCONNECTED      ErrorCode = "CHANNEL_DISCONNECTED"

	// ── Post ─────────────────────────────────────────────────────────────────
	ErrCodeINVALID_POST_CAPTION   ErrorCode = "INVALID_POST_CAPTION"
	ErrCodeCAPTION_TOO_LONG       ErrorCode = "CAPTION_TOO_LONG"
	ErrCodeNO_CHANNELS_SELECTED   ErrorCode = "NO_CHANNELS_SELECTED"
	ErrCodePOST_NOT_FOUND         ErrorCode = "POST_NOT_FOUND"
	ErrCodePOST_ALREADY_PUBLISHED ErrorCode = "POST_ALREADY_PUBLISHED"
	ErrCodeINVALID_STATUS_TRANSITION ErrorCode = "INVALID_STATUS_TRANSITION"
	ErrCodeCONCURRENCY_CONFLICT      ErrorCode = "CONCURRENCY_CONFLICT"
	ErrCodeSCHEDULE_DATE_IN_PAST  ErrorCode = "SCHEDULE_DATE_IN_PAST"
	ErrCodeCANNOT_EDIT_POST       ErrorCode = "CANNOT_EDIT_POST"
	ErrCodeCANNOT_DELETE_POST     ErrorCode = "CANNOT_DELETE_POST"

	// ── Media ────────────────────────────────────────────────────────────────
	ErrCodeINVALID_FILE_TYPE  ErrorCode = "INVALID_FILE_TYPE"
	ErrCodeINVALID_MEDIA_TYPE ErrorCode = "INVALID_MEDIA_TYPE"
	ErrCodeFILE_TOO_LARGE     ErrorCode = "FILE_TOO_LARGE"
	ErrCodeVIRUS_DETECTED     ErrorCode = "VIRUS_DETECTED"

	// ── Permission (générique) ───────────────────────────────────────────────
	ErrCodePERMISSION_DENIED ErrorCode = "PERMISSION_DENIED"
	ErrCodeINSUFFICIENT_ROLE ErrorCode = "INSUFFICIENT_ROLE"

	// ── System / Validation générique ────────────────────────────────────────
	ErrCodeVALIDATION_FAILED      ErrorCode = "VALIDATION_FAILED"
	ErrCodeINVALID_INPUT          ErrorCode = "INVALID_INPUT"
	ErrCodeMISSING_REQUIRED_FIELD ErrorCode = "MISSING_REQUIRED_FIELD"
	ErrCodeINVALID_EMAIL          ErrorCode = "INVALID_EMAIL"
	ErrCodeEMAIL_ALREADY_EXISTS   ErrorCode = "EMAIL_ALREADY_EXISTS"
	ErrCodePASSWORD_TOO_WEAK      ErrorCode = "PASSWORD_TOO_WEAK"
	ErrCodePLAN_LIMIT_EXCEEDED    ErrorCode = "PLAN_LIMIT_EXCEEDED"
	ErrCodeMAX_MEMBERS_REACHED    ErrorCode = "MAX_MEMBERS_REACHED"
	ErrCodeMAX_BRANDS_REACHED     ErrorCode = "MAX_BRANDS_REACHED"
	ErrCodeMAX_CHANNELS_REACHED   ErrorCode = "MAX_CHANNELS_REACHED"
	ErrCodeINVARIANT_VIOLATION    ErrorCode = "INVARIANT_VIOLATION"
	ErrCodeRATE_LIMIT_EXCEEDED    ErrorCode = "RATE_LIMIT_EXCEEDED"

	// ── Infrastructure ───────────────────────────────────────────────────────
	// DATABASE_CONNECTION_FAILED : valeur string intentionnellement différente du nom de constante
	ErrCodeDATABASE_CONNECTION      ErrorCode = "DATABASE_CONNECTION_FAILED"
	ErrCodeDATABASE_TIMEOUT         ErrorCode = "DATABASE_TIMEOUT"
	// DATABASE_CONSTRAINT_VIOLATED : valeur string intentionnellement différente du nom de constante
	ErrCodeDATABASE_CONSTRAINT      ErrorCode = "DATABASE_CONSTRAINT_VIOLATED"
	ErrCodeEXTERNAL_API_TIMEOUT     ErrorCode = "EXTERNAL_API_TIMEOUT"
	ErrCodeEXTERNAL_API_UNAUTHORIZED ErrorCode = "EXTERNAL_API_UNAUTHORIZED"
	ErrCodeEXTERNAL_API_RATE_LIMIT  ErrorCode = "EXTERNAL_API_RATE_LIMIT"
	ErrCodeFACEBOOK_API_ERROR       ErrorCode = "FACEBOOK_API_ERROR"
	ErrCodeINSTAGRAM_API_ERROR      ErrorCode = "INSTAGRAM_API_ERROR"
	ErrCodeLINKEDIN_API_ERROR       ErrorCode = "LINKEDIN_API_ERROR"
	ErrCodeBILLING_SERVICE_FAILED   ErrorCode = "BILLING_SERVICE_FAILED"
	ErrCodeSTRIPE_API_ERROR         ErrorCode = "STRIPE_API_ERROR"
	ErrCodeINTERNAL_SERVER_ERROR    ErrorCode = "INTERNAL_SERVER_ERROR"
	ErrCodeSERVICE_UNAVAILABLE      ErrorCode = "SERVICE_UNAVAILABLE"

	// ── Security / RGPD ──────────────────────────────────────────────────────
	ErrCodeCONSENT_REQUIRED         ErrorCode = "CONSENT_REQUIRED"
	ErrCodeCONSENT_WITHDRAWN        ErrorCode = "CONSENT_WITHDRAWN"
	ErrCodeDATA_RETENTION_VIOLATION ErrorCode = "DATA_RETENTION_VIOLATION"
	ErrCodeDATA_EXPORT_FAILED       ErrorCode = "DATA_EXPORT_FAILED"
	ErrCodeSECURITY_VIOLATION       ErrorCode = "SECURITY_VIOLATION"

	// ── Approval / Workflow ──────────────────────────────────────────────────
	ErrCodeAPPROVAL_NOT_FOUND              ErrorCode = "APPROVAL_NOT_FOUND"
	ErrCodeNOT_POST_AUTHOR                 ErrorCode = "NOT_POST_AUTHOR"
	ErrCodeAPPROVAL_DECISION_IMMUTABLE     ErrorCode = "APPROVAL_DECISION_IMMUTABLE"

	// ── Generic ──────────────────────────────────────────────────────────────
	ErrCodeUSER_NOT_FOUND             ErrorCode = "USER_NOT_FOUND"
	ErrCodeNOT_FOUND                  ErrorCode = "NOT_FOUND"
	ErrCodeINSUFFICIENT_PERMISSIONS   ErrorCode = "INSUFFICIENT_PERMISSIONS"
	ErrCodeVALIDATION                 ErrorCode = "VALIDATION"
	ErrCodeINTERNAL                   ErrorCode = "INTERNAL"
	ErrCodeFORBIDDEN                  ErrorCode = "FORBIDDEN"
	ErrCodeCHANNEL_ERROR              ErrorCode = "CHANNEL_ERROR"

	// ── Invitation ───────────────────────────────────────────────────────────
	ErrCodeCANNOT_INVITE_HIGHER_ROLE  ErrorCode = "CANNOT_INVITE_HIGHER_ROLE"
	ErrCodeMEMBER_LIMIT_REACHED       ErrorCode = "MEMBER_LIMIT_REACHED"
	ErrCodeINVITATION_ALREADY_PENDING ErrorCode = "INVITATION_ALREADY_PENDING"
	ErrCodeALREADY_MEMBER             ErrorCode = "ALREADY_MEMBER"
	ErrCodeINVITATION_NOT_FOUND       ErrorCode = "INVITATION_NOT_FOUND"
	ErrCodeINVITATION_NOT_PENDING     ErrorCode = "INVITATION_NOT_PENDING"
	ErrCodeINVITATION_EXPIRED         ErrorCode = "INVITATION_EXPIRED"
	ErrCodeINVITATION_EMAIL_MISMATCH  ErrorCode = "INVITATION_EMAIL_MISMATCH"

	// ── Brand (extra) ─────────────────────────────────────────────────────────
	ErrCodeBRAND_LIMIT_REACHED      ErrorCode = "BRAND_LIMIT_REACHED"
	ErrCodeBRAND_SLUG_ALREADY_EXISTS ErrorCode = "BRAND_SLUG_ALREADY_EXISTS"

	// ── Stub / not yet implemented ─────────────────────────────────────────
	ErrCodeNOT_IMPLEMENTED ErrorCode = "NOT_IMPLEMENTED"
)

// ErrNotFound — sentinel returned by repo adapters when a record is not found.
// Use errors.Is(err, domain.ErrNotFound) in use-case layer.
var ErrNotFound = fmt.Errorf("not found")

// ---------------------------------------------------------------------------
// 2. Severity
// ---------------------------------------------------------------------------

// Severity représente la gravité d'une erreur.
type Severity string

const (
	SeverityLOW      Severity = "LOW"
	SeverityMEDIUM   Severity = "MEDIUM"
	SeverityHIGH     Severity = "HIGH"
	SeverityCRITICAL Severity = "CRITICAL"
)

// ---------------------------------------------------------------------------
// 3. DomainError
// ---------------------------------------------------------------------------

// DomainError représente une erreur métier ou technique enrichie avec des métadonnées.
// Implémente l'interface error de Go.
type DomainError struct {
	Code      ErrorCode
	Message   string
	Details   map[string]interface{}
	Severity  Severity
	Retryable bool
	Timestamp time.Time
}

// Error implémente l'interface error.
func (e *DomainError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewDomainError est le constructeur de base pour DomainError.
func NewDomainError(
	code ErrorCode,
	message string,
	details map[string]interface{},
	severity Severity,
	retryable bool,
) *DomainError {
	return &DomainError{
		Code:      code,
		Message:   message,
		Details:   details,
		Severity:  severity,
		Retryable: retryable,
		Timestamp: time.Now().UTC(),
	}
}

// ---------------------------------------------------------------------------
// 4. Result[T] — generics Go 1.21+
// ---------------------------------------------------------------------------

// Result[T] encapsule soit une valeur de succès (T), soit une erreur métier.
// Champs privés — accès uniquement via accesseurs (DA-1 Option A).
// Utiliser Ok() et Fail() pour construire.
type Result[T any] struct {
	value T
	err   *DomainError
}

// Ok crée un Result de succès.
func Ok[T any](val T) Result[T] {
	return Result[T]{value: val}
}

// Fail crée un Result d'échec.
func Fail[T any](err *DomainError) Result[T] {
	return Result[T]{err: err}
}

// Value retourne la valeur de succès.
// Panique si le Result est un échec — vérifier IsOk() en premier.
func (r Result[T]) Value() T {
	if r.err != nil {
		panic("Result.Value() called on failure — check IsOk() first")
	}
	return r.value
}

// Err retourne l'erreur métier, ou nil si succès.
func (r Result[T]) Err() *DomainError {
	return r.err
}

// IsOk retourne true si le Result contient une valeur valide.
func (r Result[T]) IsOk() bool {
	return r.err == nil
}

// IsFail retourne true si le Result contient une erreur.
func (r Result[T]) IsFail() bool {
	return r.err != nil
}

// ---------------------------------------------------------------------------
// 5. errorMessages — messages user-friendly par ErrorCode (Phase 0 §3.3)
// ---------------------------------------------------------------------------

var errorMessages = map[ErrorCode]string{
	// Validation
	ErrCodeINVALID_INPUT:             "Les données fournies sont invalides",
	ErrCodeSLUG_ALREADY_EXISTS:       "Ce nom d'URL est déjà utilisé",
	ErrCodeINVALID_EMAIL:             "L'adresse email est invalide",
	ErrCodePASSWORD_TOO_WEAK:         "Le mot de passe ne respecte pas les exigences de sécurité",

	// Permission
	ErrCodePERMISSION_DENIED:         "Vous n'avez pas la permission d'effectuer cette action",
	ErrCodeINSUFFICIENT_ROLE:         "Votre rôle ne permet pas cette action",
	ErrCodeBRAND_ACCESS_DENIED:       "Vous n'avez pas accès à cette marque",

	// Business Logic
	ErrCodePLAN_LIMIT_EXCEEDED:       "La limite de votre abonnement est atteinte",
	ErrCodeWORKSPACE_NOT_FOUND:       "Workspace introuvable",
	ErrCodeDUPLICATE_FREE_WORKSPACE:  "Vous avez déjà un workspace gratuit",
	ErrCodeINVARIANT_VIOLATION:       "Une règle métier a été violée",
	ErrCodeINVALID_STATUS_TRANSITION: "Cette transition d'état n'est pas autorisée",
	ErrCodeCONCURRENCY_CONFLICT:      "La ressource a été modifiée concurremment — veuillez recharger et réessayer",
	ErrCodeSCHEDULE_DATE_IN_PAST:     "La date de planification ne peut pas être dans le passé",

	// Infrastructure
	ErrCodeDATABASE_CONNECTION:       "Erreur de connexion à la base de données",
	ErrCodeEXTERNAL_API_TIMEOUT:      "Le service externe ne répond pas",
	ErrCodeTOKEN_EXPIRED:             "Votre session a expiré",

	// RGPD / Security
	ErrCodeCONSENT_REQUIRED:          "Votre consentement est requis pour cette action",
	ErrCodeDATA_RETENTION_VIOLATION:  "La durée de conservation des données est dépassée",
	ErrCodeEMAIL_NOT_VERIFIED:        "L'adresse email doit être vérifiée avant d'effectuer cette action",
	ErrCodeINVALID_TOKEN:             "Token invalide ou expiré",
}

// ---------------------------------------------------------------------------
// 6. httpStatusMap — mapping ErrorCode → HTTP status (Phase 0 §8.1)
// ---------------------------------------------------------------------------

var httpStatusMap = map[ErrorCode]int{
	// 400 Bad Request — validation
	ErrCodeVALIDATION_FAILED:          400,
	ErrCodeINVALID_INPUT:              400,
	ErrCodeMISSING_REQUIRED_FIELD:     400,
	ErrCodeINVALID_WORKSPACE_NAME:     400,
	ErrCodeSLUG_INVALID_FORMAT:        400,
	ErrCodeINVALID_EMAIL:              400,
	ErrCodePASSWORD_TOO_WEAK:          400,
	ErrCodeINVALID_POST_CAPTION:       400,
	ErrCodeCAPTION_TOO_LONG:           400,
	ErrCodeNO_CHANNELS_SELECTED:       400,
	ErrCodeINVALID_FILE_TYPE:          400,
	ErrCodeFILE_TOO_LARGE:             400,
	ErrCodeSCHEDULE_DATE_IN_PAST:      400,
	ErrCodeINVALID_BRAND_NAME:         400, // A1
	ErrCodeBRAND_NAME_TOO_LONG:        400, // A1

	// 401 Unauthorized — authentification
	ErrCodeAUTHENTICATION_FAILED:      401,
	ErrCodeINVALID_CREDENTIALS:        401,
	ErrCodeTOKEN_EXPIRED:              401,
	ErrCode2FA_REQUIRED:               401,
	ErrCodeINVALID_2FA_CODE:           401,
	ErrCodeINVALID_TOKEN:              401,
	ErrCodeINVALID_OAUTH_TOKEN:        401, // A1

	// 402 Payment Required — limites de plan
	ErrCodePLAN_LIMIT_EXCEEDED:        402,
	ErrCodeMAX_MEMBERS_REACHED:        402,
	ErrCodeMAX_BRANDS_REACHED:         402,
	ErrCodeMAX_CHANNELS_REACHED:       402,

	// 403 Forbidden — permissions refusées
	ErrCodePERMISSION_DENIED:          403,
	ErrCodeINSUFFICIENT_ROLE:          403,
	ErrCodeWORKSPACE_ACCESS_DENIED:    403,
	ErrCodeBRAND_ACCESS_DENIED:        403,
	ErrCodeCHANNEL_ACCESS_DENIED:      403,
	ErrCodeCHANNEL_PUBLISH_DENIED:     403,
	ErrCodeCANNOT_EDIT_POST:           403,
	ErrCodeCANNOT_DELETE_POST:         403,
	ErrCodeNOT_WORKSPACE_OWNER:        403,
	ErrCodeACCOUNT_LOCKED:             403,
	ErrCodeEMAIL_NOT_VERIFIED:         403,
	ErrCodeNO_ASSIGNMENT_TO_BRAND:     403, // A1

	// 404 Not Found — ressource introuvable
	ErrCodeWORKSPACE_NOT_FOUND:        404,
	ErrCodeMEMBER_NOT_FOUND:           404,
	ErrCodeBRAND_NOT_FOUND:            404,
	ErrCodeCHANNEL_NOT_FOUND:          404,
	ErrCodePOST_NOT_FOUND:             404,

	// 409 Conflict — conflits d'état
	ErrCodeSLUG_ALREADY_EXISTS:        409,
	ErrCodeEMAIL_ALREADY_EXISTS:       409,
	ErrCodeMEMBER_ALREADY_EXISTS:      409,
	ErrCodeCHANNEL_ALREADY_CONNECTED:  409,
	ErrCodePOST_ALREADY_PUBLISHED:     409,
	ErrCodeBRAND_ALREADY_ARCHIVED:     409,
	ErrCodeDUPLICATE_FREE_WORKSPACE:   409,
	ErrCodeINVALID_STATUS_TRANSITION:  409,
	ErrCodeCONCURRENCY_CONFLICT:       409,

	// 410 Gone — ressource supprimée définitivement
	ErrCodeWORKSPACE_ALREADY_DELETED:  410, // A1

	// 422 Unprocessable Entity — erreurs métier
	ErrCodeINVARIANT_VIOLATION:            422,
	ErrCodeWORKSPACE_MUST_HAVE_OWNER:      422,
	ErrCodeCANNOT_REMOVE_OWNER:            422,
	ErrCodeCANNOT_DELETE_ACTIVE_WORKSPACE: 422,
	ErrCodeVIRUS_DETECTED:                 422,

	// 429 Too Many Requests — rate limiting
	ErrCodeRATE_LIMIT_EXCEEDED:        429,
	ErrCodeTOO_MANY_ATTEMPTS:          429,
	ErrCodeEXTERNAL_API_RATE_LIMIT:    429, // A1

	// 403 Forbidden — security violation
	ErrCodeSECURITY_VIOLATION:          403,
	ErrCodeNOT_POST_AUTHOR:             403,

	// 404 Not Found — approval
	ErrCodeAPPROVAL_NOT_FOUND:          404,

	// 409 Conflict — approval workflow
	ErrCodeAPPROVAL_DECISION_IMMUTABLE: 409,

	// 451 Unavailable For Legal Reasons — RGPD
	ErrCodeCONSENT_REQUIRED:           451,
	ErrCodeCONSENT_WITHDRAWN:          451,
	ErrCodeDATA_RETENTION_VIOLATION:   451,

	// 501 Not Implemented
	ErrCodeNOT_IMPLEMENTED: 501,

	// 500 Internal Server Error
	ErrCodeINTERNAL_SERVER_ERROR:      500,
	ErrCodeDATABASE_CONNECTION:        500,
	ErrCodeDATABASE_TIMEOUT:           500,
	ErrCodeDATABASE_CONSTRAINT:        500, // A1
	ErrCodeDATA_EXPORT_FAILED:         500, // A1

	// 502 Bad Gateway — erreurs API externes
	ErrCodeEXTERNAL_API_TIMEOUT:       502,
	ErrCodeFACEBOOK_API_ERROR:         502,
	ErrCodeINSTAGRAM_API_ERROR:        502,
	ErrCodeLINKEDIN_API_ERROR:         502,
	ErrCodeSTRIPE_API_ERROR:           502,
	ErrCodeEXTERNAL_API_UNAUTHORIZED:  502, // A1

	// 503 Service Unavailable
	ErrCodeSERVICE_UNAVAILABLE:        503,
	ErrCodeBILLING_SERVICE_FAILED:     503,
}

// ---------------------------------------------------------------------------
// 7. retryableErrors — mapping ErrorCode → bool (Phase 0 §4.3)
// ---------------------------------------------------------------------------

var retryableErrors = map[ErrorCode]bool{
	// Retryable : erreurs temporaires
	ErrCodeDATABASE_TIMEOUT:        true,
	ErrCodeDATABASE_CONNECTION:     true,
	ErrCodeEXTERNAL_API_TIMEOUT:    true,
	ErrCodeEXTERNAL_API_RATE_LIMIT: true,
	ErrCodeSERVICE_UNAVAILABLE:     true,
	ErrCodeCONCURRENCY_CONFLICT:    true,

	// Non-retryable : erreurs permanentes
	ErrCodeVALIDATION_FAILED:       false,
	ErrCodePERMISSION_DENIED:       false,
	ErrCodeBRAND_ACCESS_DENIED:     false,
	ErrCodeSLUG_ALREADY_EXISTS:     false,
	ErrCodeINVALID_CREDENTIALS:     false,
	ErrCodeTOKEN_EXPIRED:           false,
	ErrCodeINVARIANT_VIOLATION:     false,
	ErrCodeEMAIL_NOT_VERIFIED:      false,
	ErrCodeINVALID_TOKEN:           false,
}

// ---------------------------------------------------------------------------
// 8. Fonctions utilitaires
// ---------------------------------------------------------------------------

// GetHTTPStatus retourne le status HTTP associé à un ErrorCode.
// Fallback : 500 (fail secure).
func GetHTTPStatus(code ErrorCode) int {
	if status, ok := httpStatusMap[code]; ok {
		return status
	}
	return 500
}

// IsRetryable retourne true si l'erreur associée au code est retryable.
// Défaut : false (fail secure).
func IsRetryable(code ErrorCode) bool {
	return retryableErrors[code]
}

// GetMessage retourne le message user-friendly associé à un ErrorCode.
// Fallback : message générique si le code est inconnu.
func GetMessage(code ErrorCode) string {
	if msg, ok := errorMessages[code]; ok {
		return msg
	}
	return "Une erreur inattendue s'est produite"
}
