package workingset

type OutputFormat string

const (
	OutputFormatJSON          OutputFormat = "json"
	OutputFormatYAML          OutputFormat = "yaml"
	OutputFormatHumanReadable OutputFormat = "human"
)

var supportedFormats = []OutputFormat{OutputFormatJSON, OutputFormatYAML, OutputFormatHumanReadable}

func SupportedFormats() []string {
	formats := make([]string, len(supportedFormats))
	for i, v := range supportedFormats {
		formats[i] = string(v)
	}
	return formats
}
