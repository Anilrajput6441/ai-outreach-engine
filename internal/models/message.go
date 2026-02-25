package models

type HRMessage struct {
	HRName      string `json:"hr_name"`
	HREmail     string `json:"hr_email"`
	CompanyName string `json:"company_name"`
	Website     string `json:"website"`
}
type EmailMessage struct {
	HREmail     string `json:"hr_email"`
	CompanyName string `json:"company_name"`
	Subject     string `json:"subject"`
	Body        string `json:"body"`
}
