package service

import (
	"fmt"
	"net/smtp"
	"time"

	"yourapp/internal/config"
)

// EmailService mendefinisikan antarmuka untuk layanan pengiriman email.
type EmailService interface {
	SendOTPEmail(to, otpCode string) error
	SendResetPasswordEmail(to, resetLink string) error
	SendVerificationEmail(to, token string) error
	SendWelcomeEmail(to, name string) error
}

type emailService struct {
	config *config.Config
}

// NewEmailService membuat instance baru dari EmailService.
func NewEmailService(cfg *config.Config) EmailService {
	return &emailService{
		config: cfg,
	}
}

// sendEmail adalah helper untuk mengirim email tanpa HTML (text-only fallback).
func (s *emailService) sendEmail(to, subject, body string) error {
	return s.sendEmailHTML(to, subject, body, body)
}

// sendEmailHTML mengirim email multipart dengan versi HTML dan plain text.
func (s *emailService) sendEmailHTML(to, subject, htmlBody, textBody string) error {
	if s.config.SMTPUsername == "" || s.config.SMTPPassword == "" {
		// In development, just log the email
		fmt.Printf("[EMAIL] To: %s, Subject: %s\nBody: %s\n", to, subject, textBody)
		return nil
	}

	auth := smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)
	from := s.config.EmailFrom
	if from == "" {
		from = s.config.SMTPUsername
	}

	// Use custom email name if available
	fromHeader := from
	if s.config.EmailName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", s.config.EmailName, from)
	}

	// Create multipart message with HTML and plain text
	boundary := "----=_NextPart_" + fmt.Sprintf("%d", time.Now().UnixNano())
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n",
		fromHeader, to, subject, boundary)

	// Plain text part
	textPart := fmt.Sprintf("--%s\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n%s\r\n",
		boundary, textBody)

	// HTML part
	htmlPart := fmt.Sprintf("--%s\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n%s\r\n",
		boundary, htmlBody)

	// End boundary
	endBoundary := fmt.Sprintf("--%s--\r\n", boundary)

	msg := []byte(headers + textPart + htmlPart + endBoundary)
	addr := fmt.Sprintf("%s:%s", s.config.SMTPHost, s.config.SMTPPort)

	err := smtp.SendMail(addr, auth, from, []string{to}, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (s *emailService) SendOTPEmail(to, otpCode string) error {
	subject := "Kode Verifikasi OTP Anda"

	htmlBody := fmt.Sprintf(`
<div style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f5f7fa;">
    <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="background-color: #f5f7fa;">
        <tr>
            <td align="center" style="padding: 40px 20px;">
                <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="600" style="max-width: 600px; width: 100%%; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 6px rgba(0, 0, 0, 0.07);">
                    <!-- Header -->
                    <tr>
                        <td align="center" style="padding: 40px 40px 30px; background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); border-radius: 12px 12px 0 0;">
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                                <tr>
                                    <td align="center" style="width: 64px; height: 64px; background-color: #ffffff; border-radius: 50%%; font-size: 32px; line-height: 64px;">
                                        🔐
                                    </td>
                                </tr>
                                <tr>
                                    <td align="center" style="padding-top: 16px;">
                                        <h1 style="margin: 0; color: #ffffff; font-size: 28px; font-weight: 700; letter-spacing: -0.5px;">Kode Verifikasi</h1>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                    
                    <!-- Content -->
                    <tr>
                        <td style="padding: 40px;">
                            <p style="margin: 0 0 24px; color: #2d3748; font-size: 16px; line-height: 1.6;">
                                Halo,
                            </p>
                            <p style="margin: 0 0 32px; color: #4a5568; font-size: 15px; line-height: 1.6;">
                                Terima kasih telah mendaftar di <strong>%s</strong>. Gunakan kode OTP di bawah ini untuk memverifikasi akun Anda:
                            </p>
                            
                            <!-- OTP Code Box -->
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 32px;">
                                <tr>
                                    <td align="center" style="background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); border-radius: 12px; padding: 32px;">
                                        <div style="font-size: 13px; color: #ffffff; opacity: 0.9; margin-bottom: 12px; text-transform: uppercase; letter-spacing: 1px; font-weight: 600;">
                                            Kode Verifikasi Anda
                                        </div>
                                        <div style="font-size: 42px; font-weight: 700; color: #ffffff; letter-spacing: 8px; font-family: 'Courier New', monospace;">
                                            %s
                                        </div>
                                    </td>
                                </tr>
                            </table>
                            
                            <!-- Warning Box -->
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 24px;">
                                <tr>
                                    <td style="background-color: #fef5e7; border-left: 4px solid #f39c12; padding: 16px 20px; border-radius: 6px;">
                                        <p style="margin: 0; color: #7d6608; font-size: 14px; line-height: 1.5;">
                                            <strong>⚠️ Penting:</strong> Kode ini hanya berlaku selama <strong>10 menit</strong>. Jangan bagikan kode ini kepada siapapun untuk keamanan akun Anda.
                                        </p>
                                    </td>
                                </tr>
                            </table>
                            
                            <p style="margin: 0; color: #718096; font-size: 14px; line-height: 1.6;">
                                Jika Anda tidak melakukan permintaan ini, silakan abaikan email ini atau hubungi tim support kami.
                            </p>
                        </td>
                    </tr>
                    
                    <!-- Footer -->
                    <tr>
                        <td style="padding: 30px 40px; background-color: #f8fafc; border-radius: 0 0 12px 12px; border-top: 1px solid #e2e8f0;">
                            <p style="margin: 0 0 8px; color: #64748b; font-size: 13px; text-align: center; line-height: 1.5;">
                                Salam hangat,<br>
                                <strong>Tim %s</strong>
                            </p>
                            <p style="margin: 16px 0 0; color: #94a3b8; font-size: 12px; text-align: center; line-height: 1.5;">
                                © %d %s. All rights reserved.<br>
                                Email ini dikirim secara otomatis, mohon jangan membalas.
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</div>
`, s.config.EmailName, otpCode, s.config.EmailName, time.Now().Year(), s.config.EmailName)

	textBody := fmt.Sprintf(`
Halo,

Terima kasih telah mendaftar di %s!

Kode OTP Anda: %s

Kode ini berlaku selama 10 menit. Jangan bagikan kode ini kepada siapapun.

Jika Anda tidak meminta kode ini, silakan abaikan email ini.

Terima kasih,
Tim %s
`, s.config.EmailName, otpCode, s.config.EmailName)

	return s.sendEmailHTML(to, subject, htmlBody, textBody)
}

func (s *emailService) SendResetPasswordEmail(to, otpCode string) error {
	subject := "Reset Password - Kode OTP"

	htmlBody := fmt.Sprintf(`
<div style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f5f7fa;">
    <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="background-color: #f5f7fa;">
        <tr>
            <td align="center" style="padding: 40px 20px;">
                <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="600" style="max-width: 600px; width: 100%%; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 6px rgba(0, 0, 0, 0.07);">
                    <!-- Header -->
                    <tr>
                        <td align="center" style="padding: 40px 40px 30px; background: linear-gradient(135deg, #f093fb 0%%, #f5576c 100%%); border-radius: 12px 12px 0 0;">
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                                <tr>
                                    <td align="center" style="width: 64px; height: 64px; background-color: #ffffff; border-radius: 50%%; font-size: 32px; line-height: 64px;">
                                        🔑
                                    </td>
                                </tr>
                                <tr>
                                    <td align="center" style="padding-top: 16px;">
                                        <h1 style="margin: 0; color: #ffffff; font-size: 28px; font-weight: 700; letter-spacing: -0.5px;">Reset Password</h1>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                    
                    <!-- Content -->
                    <tr>
                        <td style="padding: 40px;">
                            <p style="margin: 0 0 24px; color: #2d3748; font-size: 16px; line-height: 1.6;">
                                Halo,
                            </p>
                            <p style="margin: 0 0 32px; color: #4a5568; font-size: 15px; line-height: 1.6;">
                                Kami menerima permintaan untuk mereset password akun <strong>%s</strong> Anda. Gunakan kode OTP di bawah ini untuk melanjutkan:
                            </p>
                            
                            <!-- OTP Code Box -->
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 32px;">
                                <tr>
                                    <td align="center" style="background: linear-gradient(135deg, #f093fb 0%%, #f5576c 100%%); border-radius: 12px; padding: 32px;">
                                        <div style="font-size: 13px; color: #ffffff; opacity: 0.9; margin-bottom: 12px; text-transform: uppercase; letter-spacing: 1px; font-weight: 600;">
                                            Kode Reset Password
                                        </div>
                                        <div style="font-size: 42px; font-weight: 700; color: #ffffff; letter-spacing: 8px; font-family: 'Courier New', monospace;">
                                            %s
                                        </div>
                                    </td>
                                </tr>
                            </table>
                            
                            <!-- Steps Box -->
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 24px;">
                                <tr>
                                    <td style="background-color: #f0f9ff; border: 1px solid #bae6fd; border-radius: 8px; padding: 20px;">
                                        <h3 style="margin: 0 0 12px; color: #0c4a6e; font-size: 15px; font-weight: 600;">Langkah Selanjutnya:</h3>
                                        <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                                            <tr>
                                                <td style="color: #075985; font-size: 14px; line-height: 1.8; padding: 4px 0;">
                                                    1. Masukkan kode OTP di atas
                                                </td>
                                            </tr>
                                            <tr>
                                                <td style="color: #075985; font-size: 14px; line-height: 1.8; padding: 4px 0;">
                                                    2. Buat password baru yang kuat
                                                </td>
                                            </tr>
                                            <tr>
                                                <td style="color: #075985; font-size: 14px; line-height: 1.8; padding: 4px 0;">
                                                    3. Login dengan password baru Anda
                                                </td>
                                            </tr>
                                        </table>
                                    </td>
                                </tr>
                            </table>
                            
                            <!-- Warning Box -->
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 24px;">
                                <tr>
                                    <td style="background-color: #fef5e7; border-left: 4px solid #f39c12; padding: 16px 20px; border-radius: 6px;">
                                        <p style="margin: 0; color: #7d6608; font-size: 14px; line-height: 1.5;">
                                            <strong>⚠️ Penting:</strong> Kode ini hanya berlaku selama <strong>10 menit</strong>. Jangan bagikan kode ini kepada siapapun.
                                        </p>
                                    </td>
                                </tr>
                            </table>
                            
                            <p style="margin: 0; color: #718096; font-size: 14px; line-height: 1.6;">
                                Jika Anda tidak melakukan permintaan ini, akun Anda tetap aman. Silakan abaikan email ini.
                            </p>
                        </td>
                    </tr>
                    
                    <!-- Footer -->
                    <tr>
                        <td style="padding: 30px 40px; background-color: #f8fafc; border-radius: 0 0 12px 12px; border-top: 1px solid #e2e8f0;">
                            <p style="margin: 0 0 8px; color: #64748b; font-size: 13px; text-align: center; line-height: 1.5;">
                                Salam hangat,<br>
                                <strong>Tim %s</strong>
                            </p>
                            <p style="margin: 16px 0 0; color: #94a3b8; font-size: 12px; text-align: center; line-height: 1.5;">
                                © %d %s. All rights reserved.<br>
                                Email ini dikirim secara otomatis, mohon jangan membalas.
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</div>
`, s.config.EmailName, otpCode, s.config.EmailName, time.Now().Year(), s.config.EmailName)

	textBody := fmt.Sprintf(`
Halo,

Kami menerima permintaan untuk mereset password akun %s Anda.

Kode OTP Reset Password: %s

Langkah selanjutnya:
1. Masukkan kode OTP di atas
2. Buat password baru yang kuat
3. Login dengan password baru Anda

Kode ini berlaku selama 10 menit. Jangan bagikan kode ini kepada siapapun.

Jika Anda tidak meminta reset password ini, silakan abaikan email ini.

Terima kasih,
Tim %s
`, s.config.EmailName, otpCode, s.config.EmailName)

	return s.sendEmailHTML(to, subject, htmlBody, textBody)
}

func (s *emailService) SendVerificationEmail(to, token string) error {
	subject := "Verifikasi Email Anda"
	verificationURL := fmt.Sprintf("%s/auth/verify-email?token=%s", s.config.ClientURL, token)

	htmlBody := fmt.Sprintf(`
<div style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f5f7fa;">
    <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="background-color: #f5f7fa;">
        <tr>
            <td align="center" style="padding: 40px 20px;">
                <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="600" style="max-width: 600px; width: 100%%; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 6px rgba(0, 0, 0, 0.07);">
                    <!-- Header -->
                    <tr>
                        <td align="center" style="padding: 40px 40px 30px; background: linear-gradient(135deg, #4facfe 0%%, #00f2fe 100%%); border-radius: 12px 12px 0 0;">
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                                <tr>
                                    <td align="center" style="width: 64px; height: 64px; background-color: #ffffff; border-radius: 50%%; font-size: 32px; line-height: 64px;">
                                        ✉️
                                    </td>
                                </tr>
                                <tr>
                                    <td align="center" style="padding-top: 16px;">
                                        <h1 style="margin: 0; color: #ffffff; font-size: 28px; font-weight: 700; letter-spacing: -0.5px;">Verifikasi Email</h1>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                    
                    <!-- Content -->
                    <tr>
                        <td style="padding: 40px;">
                            <p style="margin: 0 0 24px; color: #2d3748; font-size: 16px; line-height: 1.6;">
                                Halo,
                            </p>
                            <p style="margin: 0 0 32px; color: #4a5568; font-size: 15px; line-height: 1.6;">
                                Terima kasih telah mendaftar di <strong>%s</strong>! Untuk mengaktifkan akun Anda, silakan verifikasi alamat email dengan mengklik tombol di bawah ini:
                            </p>
                            
                            <!-- CTA Button -->
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 32px;">
                                <tr>
                                    <td align="center">
                                        <a href="%s" style="display: inline-block; padding: 16px 40px; background: linear-gradient(135deg, #4facfe 0%%, #00f2fe 100%%); color: #ffffff; text-decoration: none; border-radius: 8px; font-weight: 600; font-size: 16px; letter-spacing: 0.5px;">
                                            ✓ Verifikasi Email Saya
                                        </a>
                                    </td>
                                </tr>
                            </table>
                            
                            <!-- Alternative Link -->
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 24px;">
                                <tr>
                                    <td style="background-color: #f8fafc; border: 1px solid #e2e8f0; border-radius: 8px; padding: 20px;">
                                        <p style="margin: 0 0 12px; color: #475569; font-size: 13px; font-weight: 600;">
                                            Atau salin link berikut ke browser Anda:
                                        </p>
                                        <p style="margin: 0; color: #3b82f6; font-size: 13px; word-break: break-all; line-height: 1.5;">
                                            %s
                                        </p>
                                    </td>
                                </tr>
                            </table>
                            
                            <!-- Warning Box -->
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 24px;">
                                <tr>
                                    <td style="background-color: #fef5e7; border-left: 4px solid #f39c12; padding: 16px 20px; border-radius: 6px;">
                                        <p style="margin: 0; color: #7d6608; font-size: 14px; line-height: 1.5;">
                                            <strong>⚠️ Perhatian:</strong> Link verifikasi ini berlaku selama <strong>24 jam</strong>. Setelah itu, Anda perlu meminta link verifikasi baru.
                                        </p>
                                    </td>
                                </tr>
                            </table>
                            
                            <p style="margin: 0; color: #718096; font-size: 14px; line-height: 1.6;">
                                Jika Anda tidak mendaftar untuk akun ini, silakan abaikan email ini.
                            </p>
                        </td>
                    </tr>
                    
                    <!-- Footer -->
                    <tr>
                        <td style="padding: 30px 40px; background-color: #f8fafc; border-radius: 0 0 12px 12px; border-top: 1px solid #e2e8f0;">
                            <p style="margin: 0 0 8px; color: #64748b; font-size: 13px; text-align: center; line-height: 1.5;">
                                Salam hangat,<br>
                                <strong>Tim %s</strong>
                            </p>
                            <p style="margin: 16px 0 0; color: #94a3b8; font-size: 12px; text-align: center; line-height: 1.5;">
                                © %d %s. All rights reserved.<br>
                                Email ini dikirim secara otomatis, mohon jangan membalas.
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</div>
`, s.config.EmailName, verificationURL, verificationURL, s.config.EmailName, time.Now().Year(), s.config.EmailName)

	textBody := fmt.Sprintf(`
Halo,

Terima kasih telah mendaftar di %s!

Klik link berikut untuk memverifikasi email Anda:
%s

Link ini akan kedaluwarsa dalam 24 jam.

Jika Anda tidak meminta verifikasi ini, abaikan email ini.

Terima kasih,
Tim %s
`, s.config.EmailName, verificationURL, s.config.EmailName)

	return s.sendEmailHTML(to, subject, htmlBody, textBody)
}

func (s *emailService) SendWelcomeEmail(to, name string) error {
	subject := "Selamat Datang di " + s.config.EmailName

	htmlBody := fmt.Sprintf(`
<div style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f5f7fa;">
    <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="background-color: #f5f7fa;">
        <tr>
            <td align="center" style="padding: 40px 20px;">
                <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="600" style="max-width: 600px; width: 100%%; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 6px rgba(0, 0, 0, 0.07);">
                    <!-- Header -->
                    <tr>
                        <td align="center" style="padding: 40px 40px 30px; background: linear-gradient(135deg, #a8edea 0%%, #fed6e3 100%%); border-radius: 12px 12px 0 0;">
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                                <tr>
                                    <td align="center" style="width: 64px; height: 64px; background-color: #ffffff; border-radius: 50%%; font-size: 32px; line-height: 64px;">
                                        🎉
                                    </td>
                                </tr>
                                <tr>
                                    <td align="center" style="padding-top: 16px;">
                                        <h1 style="margin: 0; color: #1a202c; font-size: 28px; font-weight: 700; letter-spacing: -0.5px;">Selamat Datang!</h1>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                    
                    <!-- Content -->
                    <tr>
                        <td style="padding: 40px;">
                            <p style="margin: 0 0 24px; color: #2d3748; font-size: 16px; line-height: 1.6;">
                                Halo <strong>%s</strong>,
                            </p>
                            <p style="margin: 0 0 32px; color: #4a5568; font-size: 15px; line-height: 1.6;">
                                Terima kasih telah bergabung dengan <strong>%s</strong>! Kami sangat senang menyambut Anda sebagai bagian dari komunitas kami.
                            </p>
                            
                            <!-- Features Box -->
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 32px;">
                                <tr>
                                    <td style="background: linear-gradient(135deg, #f6f8fb 0%%, #f1f4f9 100%%); border: 1px solid #e2e8f0; border-radius: 12px; padding: 28px;">
                                        <h3 style="margin: 0 0 16px; color: #1e293b; font-size: 17px; font-weight: 600;">Apa yang bisa Anda lakukan:</h3>
                                        
                                        <table role="presentation" cellpadding="0" cellspacing="0" border="0" style="width: 100%%;">
                                            <tr>
                                                <td style="padding: 8px 0;">
                                                    <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                                                        <tr>
                                                            <td style="padding-right: 12px; font-size: 20px; vertical-align: top;">✨</td>
                                                            <td style="color: #475569; font-size: 14px; line-height: 1.6;">
                                                                <strong>Nikmati semua fitur</strong> yang tersedia untuk pengalaman terbaik
                                                            </td>
                                                        </tr>
                                                    </table>
                                                </td>
                                            </tr>
                                            <tr>
                                                <td style="padding: 8px 0;">
                                                    <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                                                        <tr>
                                                            <td style="padding-right: 12px; font-size: 20px; vertical-align: top;">🚀</td>
                                                            <td style="color: #475569; font-size: 14px; line-height: 1.6;">
                                                                <strong>Jelajahi pengalaman</strong> yang menyenangkan dan bermanfaat
                                                            </td>
                                                        </tr>
                                                    </table>
                                                </td>
                                            </tr>
                                            <tr>
                                                <td style="padding: 8px 0;">
                                                    <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                                                        <tr>
                                                            <td style="padding-right: 12px; font-size: 20px; vertical-align: top;">💬</td>
                                                            <td style="color: #475569; font-size: 14px; line-height: 1.6;">
                                                                <strong>Hubungi tim support</strong> kapan saja jika ada pertanyaan
                                                            </td>
                                                        </tr>
                                                    </table>
                                                </td>
                                            </tr>
                                        </table>
                                    </td>
                                </tr>
                            </table>
                            
                            <p style="margin: 0; color: #4a5568; font-size: 15px; line-height: 1.6;">
                                Jika Anda memiliki pertanyaan atau memerlukan bantuan, jangan ragu untuk menghubungi tim dukungan kami. Kami selalu siap membantu!
                            </p>
                        </td>
                    </tr>
                    
                    <!-- Footer -->
                    <tr>
                        <td style="padding: 30px 40px; background-color: #f8fafc; border-radius: 0 0 12px 12px; border-top: 1px solid #e2e8f0;">
                            <p style="margin: 0 0 8px; color: #64748b; font-size: 13px; text-align: center; line-height: 1.5;">
                                Hormat kami,<br>
                                <strong>Tim %s</strong>
                            </p>
                            <p style="margin: 16px 0 0; color: #94a3b8; font-size: 12px; text-align: center; line-height: 1.5;">
                                © %d %s. All rights reserved.<br>
                                Email ini dikirim secara otomatis, mohon jangan membalas.
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</div>
`, name, s.config.EmailName, s.config.EmailName, time.Now().Year(), s.config.EmailName)

	textBody := fmt.Sprintf(`
Halo %s,

Selamat datang di %s!

Kami sangat senang Anda bergabung dengan komunitas kami.

Anda sekarang siap untuk mulai menjelajahi semua fitur yang kami tawarkan:
- Nikmati semua fitur yang tersedia
- Jelajahi pengalaman yang menyenangkan
- Hubungi tim support jika ada pertanyaan

Jika Anda memiliki pertanyaan atau memerlukan bantuan, jangan ragu untuk menghubungi tim dukungan kami.

Terima kasih,
Tim %s
`, name, s.config.EmailName, s.config.EmailName)

	return s.sendEmailHTML(to, subject, htmlBody, textBody)
}