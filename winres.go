package main

import (
	"github.com/tc-hib/winres"
	"log"
	"os"
)

func generateWinres() {
	sysoFile := "rsrc_windows_amd64.syso"
	if _, err := os.Stat(sysoFile); err == nil {
		return
	}

	rs := winres.ResourceSet{}

	rs.SetManifest(winres.AppManifest{
		ExecutionLevel: winres.RequireAdministrator,
	})

	out, _ := os.Create(sysoFile)
	err := rs.WriteObject(out, winres.ArchAMD64)
	if err != nil {
		log.Printf("Failed to write syso: %v", err)
	}
}
