package stub

import (
	"context"
	"log"
)

// EmailService est un stub qui logue les emails au lieu de les envoyer.
// À remplacer par une implémentation SMTP/Resend en production.
type EmailService struct{}

func NewEmailService() *EmailService { return &EmailService{} }

func (e *EmailService) SendVerificationEmail(ctx context.Context, to, token string) error {
	log.Printf("[EMAIL STUB] Verification → %s  token=%s", to, token)
	return nil
}

func (e *EmailService) SendPasswordResetEmail(ctx context.Context, to, token string) error {
	log.Printf("[EMAIL STUB] PasswordReset → %s  token=%s", to, token)
	return nil
}

func (e *EmailService) SendInvitationEmail(ctx context.Context, to, workspaceName, token string) error {
	log.Printf("[EMAIL STUB] Invitation → %s  workspace=%s  token=%s", to, workspaceName, token)
	return nil
}

func (e *EmailService) SendGuestApprovalEmail(ctx context.Context, to, postTitle, approvalToken string) error {
	log.Printf("[EMAIL STUB] GuestApproval → %s  post=%s  token=%s", to, postTitle, approvalToken)
	return nil
}
