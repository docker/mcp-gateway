package eval

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluateConstant(t *testing.T) {
	assert.Equal(t, "constant", Evaluate("constant", nil))
	assert.Empty(t, Evaluate("", nil))
}

func TestEvaluate(t *testing.T) {
	assert.Equal(t, "value0", Evaluate("{{key0}}", map[string]any{"key0": "value0"}))
	assert.Equal(t, "value1", Evaluate("{{ key1 }}", map[string]any{"key1": "value1"}))
	assert.Equal(t, "value2", Evaluate("{{key2}}", map[string]any{"key2": "value2"}))
	assert.Equal(t, "v1:v2", Evaluate("{{k1}}:{{k2}}", map[string]any{"k1": "v1", "k2": "v2"}))
}

func TestDotted(t *testing.T) {
	assert.Equal(t, "child_value0", Evaluate("{{top.key}}", map[string]any{"top": map[string]any{"key": "child_value0"}}))
	assert.Equal(t, "child_value1", Evaluate("{{top . key}}", map[string]any{"top": map[string]any{"key": "child_value1"}}))
	assert.Equal(t, "child_value2", Evaluate("{{top.key|ignored}}", map[string]any{"top": map[string]any{"key": "child_value2"}}))
}

func TestEvaluateUnknown(t *testing.T) {
	assert.Empty(t, Evaluate("{{unknown}}", nil))
	assert.Empty(t, Evaluate("{{top.unknown}}", map[string]any{"top": nil}))
}

func TestAtlassian(t *testing.T) {
	assert.Equal(t, "URL", Evaluate("{{atlassian.jira.url}}", map[string]any{"atlassian": map[string]any{"jira": map[string]any{"url": "URL", "username": "USERNAME"}}}))
}

func TestVolumes(t *testing.T) {
	assert.Equal(t, "/var/run/docker.sock:/var/run/docker.sock", Evaluate("/var/run/docker.sock:/var/run/docker.sock", nil))
	assert.Equal(t, []string{"/var/run/docker.sock:/var/run/docker.sock"}, EvaluateList([]string{"/var/run/docker.sock:/var/run/docker.sock"}, nil))
	assert.Equal(t, []string{"path1:path1", "path2:path2"}, EvaluateList([]string{"{{paths|volume|into}}"}, map[string]any{"paths": []string{"path1", "path2"}}))
	assert.Equal(t, []string{"path1", "path2"}, EvaluateList([]string{"{{paths|volume-targe|into}}"}, map[string]any{"paths": []string{"path1", "path2"}}))

	assert.Equal(t, "v:v", volume("v"))
	assert.Equal(t, `C:\test\folder:/C/test/folder`, volume(`C:\test\folder`))
	assert.Equal(t, []string{"v:v", "w:w"}, evaluate([]string{"v", "w"}, volume))
}

func TestVolumeTarget(t *testing.T) {
	assert.Equal(t, "path", Evaluate("{{paths|volume-target}}", map[string]any{"paths": "path"}))
	assert.Equal(t, "/var/run/docker.sock", Evaluate("{{paths|volume-target}}", map[string]any{"paths": "/var/run/docker.sock"}))
	assert.Equal(t, "/file", Evaluate("{{paths|volume-target}}", map[string]any{"paths": "/file"}))

	assert.Equal(t, `/C/file`, Evaluate("{{paths|volume-target}}", map[string]any{"paths": `C:\file`}))
	assert.Equal(t, `/D/parent/file`, Evaluate("{{paths|volume-target}}", map[string]any{"paths": `D:\parent\file`}))
}

func TestEvaluateInto(t *testing.T) {
	assert.Equal(t, "v", Evaluate("{{k}}", map[string]any{"k": "v"}))
	assert.Equal(t, []string{"v1", "v2"}, Evaluate("{{k}}", map[string]any{"k": []string{"v1", "v2"}}))

	assert.Equal(t, []string{"v"}, Evaluate("{{k|into}}", map[string]any{"k": "v"}))
	assert.Equal(t, []string{"v1", "v2"}, Evaluate("{{k|into}}", map[string]any{"k": []string{"v1", "v2"}}))
}

func TestEvaluateFirst(t *testing.T) {
	assert.Equal(t, "v", Evaluate("{{k|first}}", map[string]any{"k": "v"}))
	assert.Equal(t, "v1", Evaluate("{{k|first}}", map[string]any{"k": []string{"v1", "v2"}}))
	assert.Empty(t, Evaluate("{{k|first}}", map[string]any{"k": []string{}}))
	assert.Empty(t, Evaluate("{{k|first}}", map[string]any{"k": nil}))

	assert.Equal(t, []string{"v"}, Evaluate("{{k|first|into}}", map[string]any{"k": "v"}))
	assert.Equal(t, []string{"v1"}, Evaluate("{{k|first|into}}", map[string]any{"k": []string{"v1", "v2"}}))
}

func TestEvaluateLast(t *testing.T) {
	assert.Equal(t, "v", Evaluate("{{k|last}}", map[string]any{"k": "v"}))
	assert.Equal(t, "v2", Evaluate("{{k|last}}", map[string]any{"k": []string{"v1", "v2"}}))
	assert.Empty(t, Evaluate("{{k|last}}", map[string]any{"k": []string{}}))
	assert.Empty(t, Evaluate("{{k|last}}", map[string]any{"k": nil}))

	assert.Equal(t, []string{"v"}, Evaluate("{{k|last|into}}", map[string]any{"k": "v"}))
	assert.Equal(t, []string{"v2"}, Evaluate("{{k|last|into}}", map[string]any{"k": []string{"v1", "v2"}}))
}

func TestEvaluateOr(t *testing.T) {
	assert.Equal(t, "default", Evaluate("{{k|or:default}}", map[string]any{"k": nil}))
	assert.Equal(t, "default", Evaluate("{{k|or:default}}", map[string]any{"k": ""}))
	assert.Equal(t, "v", Evaluate("{{k|or:default}}", map[string]any{"k": "v"}))

	assert.Equal(t, "v", Evaluate("{{k|or:v}}", map[string]any{"k": []string{}}))
	assert.Equal(t, []string{}, Evaluate("{{k|or:[]}}", map[string]any{"k": []string{}}))
	assert.Equal(t, []string{"default"}, Evaluate("{{k|or:[default]}}", map[string]any{"k": []string{}}))
}

func TestEvaluateMountAs(t *testing.T) {
	assert.Empty(t, Evaluate("{{key|mount_as:/path}}", map[string]any{"key": nil}))
	assert.Equal(t, "/local/path:/path", Evaluate("{{key|mount_as:/path}}", map[string]any{"key": "/local/path"}))
	assert.Equal(t, "/local/logs:/logs:ro", Evaluate("{{key|mount_as:/logs:ro}}", map[string]any{"key": "/local/logs"}))

	assert.Empty(t, Evaluate("{{key|mount_as:/path}}", map[string]any{"key": []string{}}))
	assert.Equal(t, "/local/path:/path", Evaluate("{{key|mount_as:/path}}", map[string]any{"key": []string{"/local/path"}}))
	assert.Equal(t, "/local/logs:/logs:ro", Evaluate("{{key|mount_as:/logs:ro}}", map[string]any{"key": []string{"/local/logs"}}))

	assert.Equal(t, "/local/path:/path", Evaluate("{{key|mount_as: /path }}", map[string]any{"key": "/local/path"}))

	assert.Equal(t, "/local/path:/path", Evaluate("{{key|mount_as:/path}}", map[string]any{"key": []string{"/local/path", "/ignored/path"}}))
}
