package regenerate

import "errors"

var ErrInvalidOverwriteValue = errors.New("invalid --overwrite value: must be allow, deny, or ask")
