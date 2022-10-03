// Copyright © 2021 The Gomon Project.

package core

import (
	"context"
)

var (
	Ctx, Cancel = context.WithCancel(context.Background())
)
