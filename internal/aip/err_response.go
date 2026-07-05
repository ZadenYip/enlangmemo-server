package aip

// 各个语义参考 https://google.aip.dev/193#guidance
type ErrResponse struct {
	Error ErrStatus `json:"error"`
}

type ErrStatus struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Status  string   `json:"status"`
	Details []Detail `json:"details"`
}

type Detail struct {
	Reason   string            `json:"reason"`
	Metadata map[string]string `json:"metadata"`
}

func NewErrResponse() *ErrResponse {
	return &ErrResponse{
		Error: ErrStatus{
			Details: []Detail{},
		},
	}
}

func (er *ErrResponse) WithMessage(message string) *ErrResponse {
	er.Error.Message = message
	return er
}

func (er *ErrResponse) WithDetails(details []Detail) *ErrResponse {
	er.Error.Details = details
	return er
}

func (er *ErrResponse) WithCodeAndStatus(status ErrorStatus) *ErrResponse {
	er.Error.Code = status.HTTPCode()
	er.Error.Status = status.String()
	return er
}

func (er *ErrResponse) AppendDetail(detail Detail) *ErrResponse {
	er.Error.Details = append(er.Error.Details, detail)
	return er
}
