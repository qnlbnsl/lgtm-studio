package link

import (
	"context" // STUB s3-commands-tests-b
)

type commandSweepTicksKey struct{} // STUB s3-commands-tests-b

func (*State) SweepCommands(context.Context) error { return nil } // STUB s3-commands-tests-b

func (*State) RunCommandSweep(context.Context, func(error)) {} // STUB s3-commands-tests-b
