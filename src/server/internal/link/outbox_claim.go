package link

import (
	"context" // STUB s3-commands-tests-b

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1" // STUB s3-commands-tests-b
)

const claimPageSize = 32 // STUB s3-commands-tests-b

type commandCursor struct{} // STUB s3-commands-tests-b

func (*State) claimNext(context.Context, string, string, *commandCursor) (*linkv1.Command, bool, error) { // STUB s3-commands-tests-b
	return nil, false, nil // STUB s3-commands-tests-b
} // STUB s3-commands-tests-b
