package aip

type BadRequest struct {
	BadRequestViolation []FieldViolation `json:"fieldViolations"`
}

type FieldViolation struct {
	Field       string `json:"field"`
	Description string `json:"description"`
}
