package mail

import (
	"bytes"
	"embed"
	"html/template"
	"net/smtp"

	"github.com/jordan-wright/email"
	"greenlight.zzh.net/internal/config"
)

//go:embed "templates"
var templateFS embed.FS

// EmailSender wraps a *config.SMTPConfig which stores configuration for sending emails.
type EmailSender struct {
    SMTPCfg *config.SMTPConfig
}

// Send sends an email whose subject and content are read from a template file.
// Use a pointer receiver because the fields of EmailSender can be dynamically loaded.
func (sender *EmailSender) Send(to, templateFile string, data any) error {
    tmpl, err := template.New("email").ParseFS(templateFS, "templates/"+templateFile)
    if err != nil {
        return err
    }

    // Execute the named tempalte "subject", passing in the dynamic data and storing the 
    // result in a bytes.Buffer variable.
    subject := new(bytes.Buffer)
    err = tmpl.ExecuteTemplate(subject, "subject", data)
    if err != nil {
        return err
    }

    // Execute the named tempalte "plainBody", passing in the dynamic data and storing the 
    // result in a bytes.Buffer variable.
    plainBody := new(bytes.Buffer)
    err = tmpl.ExecuteTemplate(plainBody, "plainBody", data)
    if err != nil {
        return err
    }

    htmlBody := new(bytes.Buffer)
    err = tmpl.ExecuteTemplate(htmlBody, "htmlBody", data)
    if err != nil {
        return err
    }

    e := email.NewEmail()
    e.From = sender.SMTPCfg.Username // 553 Mail from must equal authorized user
    e.To = []string{to}
    e.Subject = subject.String()
    e.Text = plainBody.Bytes()
    e.HTML = htmlBody.Bytes()

    smtpAuth := smtp.PlainAuth("", sender.SMTPCfg.Username, sender.SMTPCfg.Password, sender.SMTPCfg.AuthAddress)
    return e.Send(sender.SMTPCfg.ServerAddress, smtpAuth)
}