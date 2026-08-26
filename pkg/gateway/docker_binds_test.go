package gateway

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanDockerHostPathPreservesWindowsDriveRoot(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: `D:\`, want: "d:/"},
		{input: "D:/", want: "d:/"},
		{input: `D:\data`, want: "d:/data"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := cleanDockerHostPath(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestNormalizeDockerVolumeBindPreservesWindowsDriveRoot(t *testing.T) {
	got, err := normalizeDockerVolumeBindWithRoots(
		`D:\:/D:ro`,
		[]string{"d:/"},
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, "d:/:/D:ro", got)
}
