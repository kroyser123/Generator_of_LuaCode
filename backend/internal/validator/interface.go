package validator

type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
	Output string   `json:"output"`
}

type Validator interface {
	Validate(code string) ValidationResult
}
