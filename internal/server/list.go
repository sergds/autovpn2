package server

import (
	"context"
	"fmt"
	"strings"
)

func (s *AutoVPNServer) List(updates chan string, ctx context.Context) context.Context {
	pbooks := GetAllPlaybooksFromDB(s.playbookDB)
	var pbnames []string = make([]string, 0)
	for pbname, _ := range pbooks {
		pbnames = append(pbnames, pbname)
	}
	updates <- "Playbooks (" + fmt.Sprintf("%v", len(pbooks)) + "): " + strings.Join(pbnames, ", ")
	return ctx
}
