package aip

type BadRequest struct {
	BadRequestViolation []BadRequestViolation `json:"fieldViolations"`
}

type BadRequestViolation struct {
	Field       string `json:"field"`
	Description string `json:"description"`
}
