package otp

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"strings"
	"print-pro-backend/internal/infrastructure"
	"time"
)

// OTPService handles OTP generation and validation
type OTPService struct {
	redisClient *infrastructure.RedisClient
	otpTTL      time.Duration
}

// NewOTPService creates a new OTP service
func NewOTPService(redisClient *infrastructure.RedisClient) *OTPService {
	return &OTPService{
		redisClient: redisClient,
		otpTTL:      5 * time.Minute, // OTP expires in 5 minutes
	}
}

// GenerateOTP generates a 6-digit OTP and stores it in Redis
// If an OTP already exists for this email, it returns the existing OTP to prevent duplicate emails
func (s *OTPService) GenerateOTP(ctx context.Context, email string) (string, error) {
	otpKey := fmt.Sprintf("otp:%s", email)
	
	// Check if OTP already exists to prevent duplicate generation
	log.Printf("Checking for existing OTP before generating new one")
	existingOTP, err := s.redisClient.Get(ctx, otpKey)
	if err == nil && existingOTP != "" {
		// OTP already exists - return it to prevent duplicate email
		log.Printf("Existing OTP found - reusing to prevent duplicate email generation")
		return strings.TrimSpace(existingOTP), nil
	}
	
	// No existing OTP - generate new one
	log.Printf("No existing OTP found - generating new 6-digit OTP")
	otp := generateRandomOTP(6)
	log.Printf("OTP generated successfully")
	
	// Store OTP in Redis with email as key
	log.Printf("Storing OTP in Redis")
	err = s.redisClient.Set(ctx, otpKey, otp, s.otpTTL)
	if err != nil {
		log.Printf("Failed to store OTP in Redis: %v", err)
		return "", fmt.Errorf("failed to store OTP: %w", err)
	}
	log.Printf("OTP stored in Redis successfully")
	
	return otp, nil
}

// VerifyOTP verifies if the provided OTP matches the stored OTP for the email
// After successful verification, creates a reset token that allows password reset
func (s *OTPService) VerifyOTP(ctx context.Context, email, otp string) (bool, error) {
	otpKey := fmt.Sprintf("otp:%s", email)
	
	// Trim whitespace from input OTP
	otp = strings.TrimSpace(otp)
	
	storedOTP, err := s.redisClient.Get(ctx, otpKey)
	if err != nil {
		log.Printf("OTP not found in Redis or expired")
		return false, fmt.Errorf("OTP not found or expired")
	}
	
	// Trim whitespace from stored OTP as well
	storedOTP = strings.TrimSpace(storedOTP)
	
	log.Printf("Comparing OTP - Provided length: %d, Stored length: %d", len(otp), len(storedOTP))
	
	if storedOTP != otp {
		log.Printf("OTP mismatch - Invalid OTP provided")
		return false, fmt.Errorf("invalid OTP")
	}
	
	// OTP is valid - delete it to prevent reuse
	s.redisClient.Delete(ctx, otpKey)
	
	// Create a reset token that allows password reset (valid for 10 minutes)
	resetTokenKey := fmt.Sprintf("reset_token:%s", email)
	resetToken := "verified" // Simple flag to indicate OTP was verified
	err = s.redisClient.Set(ctx, resetTokenKey, resetToken, 10*time.Minute)
	if err != nil {
		return false, fmt.Errorf("failed to create reset token: %w", err)
	}
	
	return true, nil
}

// VerifyResetToken checks if a reset token exists for the email (OTP was already verified)
func (s *OTPService) VerifyResetToken(ctx context.Context, email string) (bool, error) {
	resetTokenKey := fmt.Sprintf("reset_token:%s", email)
	_, err := s.redisClient.Get(ctx, resetTokenKey)
	if err != nil {
		return false, fmt.Errorf("reset token not found or expired")
	}
	return true, nil
}

// DeleteResetToken removes the reset token after password is reset
func (s *OTPService) DeleteResetToken(ctx context.Context, email string) {
	resetTokenKey := fmt.Sprintf("reset_token:%s", email)
	s.redisClient.Delete(ctx, resetTokenKey)
}

// MarkOTPEmailSent atomically marks that an email was sent for this OTP to prevent duplicate sends
// Uses Redis SETNX for atomic check-and-set operation
// Returns true if email was already sent (should skip sending), false if this is first send
func (s *OTPService) MarkOTPEmailSent(ctx context.Context, email string) (bool, error) {
	emailSentKey := fmt.Sprintf("otp:email_sent:%s", email)
	
	// Use atomic SETNX (SET if Not eXists) to prevent race conditions
	// This atomically checks if key exists AND sets it in one operation
	wasSet, err := s.redisClient.SetNX(ctx, emailSentKey, "sent", 2*time.Minute)
	if err != nil {
		log.Printf("WARNING: Failed to mark email as sent (non-fatal): %v", err)
		// Continue anyway - worst case is duplicate email
		return false, nil
	}
	
	if !wasSet {
		// Key already existed - email was already sent recently
		log.Printf("Email already sent for this OTP - skipping duplicate send (atomic check)")
		return true, nil
	}
	
	// Key was set (this is first send) - proceed with email
	log.Printf("Email send marked in Redis (atomic) - will prevent duplicates for 2 minutes")
	return false, nil
}

// generateRandomOTP generates a random N-digit OTP
func generateRandomOTP(length int) string {
	digits := "0123456789"
	otp := make([]byte, length)
	
	for i := range otp {
		b := make([]byte, 1)
		rand.Read(b)
		otp[i] = digits[int(b[0])%len(digits)]
	}
	
	return string(otp)
}

