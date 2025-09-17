package yq

import (
	"fmt"
	"strings"

	"github.com/mikefarah/yq/v4/pkg/yqlib"
	"gopkg.in/op/go-logging.v1"
)

var yamlPref = yqlib.YamlPreferences{
	Indent:                      2,
	ColorsEnabled:               false,
	LeadingContentPreProcessing: true,
	PrintDocSeparators:          true,
	UnwrapScalar:                true,
	EvaluateTogether:            false,
}

type logBackend struct{}

func (n logBackend) Log(logging.Level, int, *logging.Record) error {
	return nil
}

func (n logBackend) GetLevel(string) logging.Level {
	return logging.ERROR
}

func (n logBackend) SetLevel(logging.Level, string) {
}

func (n logBackend) IsEnabledFor(logging.Level, string) bool {
	return false
}

func NewYamlDecoder() yqlib.Decoder {
	return yqlib.NewYamlDecoder(yamlPref)
}

func NewYamlEncoder() yqlib.Encoder {
	return yqlib.NewYamlEncoder(yamlPref)
}

func NewJSONEncoder() yqlib.Encoder {
	pref := yqlib.JsonPreferences{
		Indent:        0,
		ColorsEnabled: false,
		UnwrapScalar:  true,
	}
	return yqlib.NewJSONEncoder(pref)
}

func Evaluate(yqExpr string, content []byte, decoder yqlib.Decoder, encoder yqlib.Encoder) ([]byte, error) {
	// Don't use the default (chatty) logger
	yqlib.GetLogger().SetBackend(logBackend{})

	evaluator := yqlib.NewStringEvaluator()
	result, err := evaluator.EvaluateAll(yqExpr, string(content), encoder, decoder)
	if err != nil {
		return nil, fmt.Errorf("EvaluateYqExpression: failed to evaluate YQ expression '%s': %w", yqExpr, err)
	}
	result = strings.TrimSpace(result)
	return []byte(result), nil
}
