package oauth

import (
	"errors"
	"io"
	"testing"

	"github.com/docker/docker-credential-helpers/client"
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type deleteTestProgram struct {
	input  string
	output []byte
	err    error
}

func (p *deleteTestProgram) Input(in io.Reader) {
	data, _ := io.ReadAll(in)
	p.input = string(data)
}

func (p *deleteTestProgram) Output() ([]byte, error) { return p.output, p.err }

func TestReadWriteHelperDeleteProtocol(t *testing.T) {
	for _, tc := range []struct {
		name         string
		getOutput    string
		getErr       error
		eraseErr     error
		wantNotFound bool
		wantErase    bool
		wantErr      bool
	}{
		{name: "absent legacy credential", getOutput: "credentials not found in native keychain", getErr: errors.New("exit status 1"), wantNotFound: true, wantErr: true},
		{name: "existing credential", getOutput: `{"Username":"test","Secret":"test"}`, wantErase: true},
		{name: "locked store", getOutput: "store is locked", getErr: errors.New("exit status 1"), wantErr: true},
		{name: "erase failure", getOutput: `{"Username":"test","Secret":"test"}`, eraseErr: errors.New("permission denied"), wantErase: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var actions []string
			get := &deleteTestProgram{output: []byte(tc.getOutput), err: tc.getErr}
			erase := &deleteTestProgram{err: tc.eraseErr}
			helper := &readWriteHelper{program: func(args ...string) client.Program {
				require.Len(t, args, 1)
				actions = append(actions, args[0])
				if args[0] == "get" {
					return get
				}
				require.Equal(t, "erase", args[0])
				return erase
			}}
			err := helper.Delete("https://auth.example/legacy")
			assert.Equal(t, tc.wantErr, err != nil)
			assert.Equal(t, tc.wantNotFound, credentials.IsErrCredentialsNotFound(err))
			assert.Equal(t, "https://auth.example/legacy", get.input)
			if tc.wantErase {
				assert.Equal(t, []string{"get", "erase"}, actions)
				assert.Equal(t, get.input, erase.input)
			} else {
				assert.Equal(t, []string{"get"}, actions)
			}
		})
	}
}
