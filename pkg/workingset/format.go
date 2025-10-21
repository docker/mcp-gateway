package workingset

type OutputFormat string

const (
	OutputFormatJSON OutputFormat = "json"
	OutputFormatYAML OutputFormat = "yaml"
)

var supportedFormats = []OutputFormat{OutputFormatJSON, OutputFormatYAML}

func SupportedFormats() []string {
	formats := make([]string, len(supportedFormats))
	for i, v := range supportedFormats {
		formats[i] = string(v)
	}
	return formats
}
