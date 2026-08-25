package mailer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/config"
	mail "github.com/wneessen/go-mail"
)

type Sender interface {
	SendHTML(ctx context.Context, to, subject, body string) error
}

type SMTP struct {
	client *mail.Client
	from   string
}

func NewSMTP(c config.MailConf) (*SMTP, error) {
	options := []mail.Option{
		mail.WithPort(c.Port),
		mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
		mail.WithUsername(c.Username),
		mail.WithPassword(c.Password),
	}

	switch strings.ToLower(c.Security) {
	case "ssl":
		options = append(options, mail.WithSSL())
	case "starttls":
		options = append(
			options,
			mail.WithTLSPortPolicy(mail.TLSMandatory),
		)
	default:
		return nil, fmt.Errorf("unsupported mail security: %s", c.Security)
	}

	client, err := mail.NewClient(c.Host, options...)
	if err != nil {
		return nil, fmt.Errorf("create mail client: %w", err)
	}

	return &SMTP{
		client: client,
		from:   c.From,
	}, nil
}

func (s *SMTP) SendHTML(
	ctx context.Context,
	to string,
	subject string,
	body string,
) error {
	message := mail.NewMsg()

	if err := message.From(s.from); err != nil {
		return fmt.Errorf("set sender: %w", err)
	}

	if err := message.To(to); err != nil {
		return fmt.Errorf("set recipient: %w", err)
	}

	message.Subject(subject)
	message.SetBodyString(mail.TypeTextHTML, body)

	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := s.client.DialAndSendWithContext(sendCtx, message); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}

	return nil
}
