package workingset

import (
	"fmt"

	"github.com/mikefarah/yq/v4/pkg/yqlib"

	"github.com/docker/mcp-gateway/pkg/yq"
)

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

func ApplyYqExpression(data []byte, format OutputFormat, yqExpr string) ([]byte, error) {
	var decoder yqlib.Decoder
	var encoder yqlib.Encoder
	switch format {
	case OutputFormatJSON:
		decoder = yqlib.NewJSONDecoder()
		encoder = yqlib.NewJSONEncoder(yqlib.JsonPreferences{
			Indent:        2,
			ColorsEnabled: false,
			UnwrapScalar:  true,
		})
	case OutputFormatYAML, OutputFormatHumanReadable:
		decoder = yqlib.NewYamlDecoder(yqlib.NewDefaultYamlPreferences())
		encoder = yqlib.NewYamlEncoder(yqlib.NewDefaultYamlPreferences())
	default:
		return nil, fmt.Errorf("unsupported input type: %s", format)
	}

	data, err := yq.Evaluate(yqExpr, data, decoder, encoder)
	if err != nil {
		return nil, err
	}
	return data, nil
}
