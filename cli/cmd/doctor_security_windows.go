package cmd

import "os"

func ownedByMe(os.FileInfo) bool { return true }
