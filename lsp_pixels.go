package main

import "errors"

var errLSPPixelsPending = errors.New("final recovered LSP pixels have not reached the owned window")
