package server

import (
	"log"
	"time"
)

// More like AutoSlow, because of how unscalable and unoptimized it is. Needs redesign and refactor like client code. Someday i'll have the time for that ~sigh~
type AutoUpdater struct {
	cronTable map[string]int   // Contains entries for auto update intervals. playbook name <==> interval in hours.
	ageTable  map[string]int64 // Contains live difference of Now and Install time Unix (seconds) timestamps. Basically playbook's age.
	server    *AutoVPNServer
}

func NewAutoUpdater(server *AutoVPNServer) *AutoUpdater {
	return &AutoUpdater{cronTable: make(map[string]int), ageTable: make(map[string]int64), server: server}
}

func (u *AutoUpdater) Tick() {
	books := GetAllPlaybooksFromDB(u.server.playbookDB)
	for k, _ := range u.cronTable {
		u.ageTable[k] = time.Now().Unix() - books[k].InstallTime
		//log.Println(k, v, u.ageTable[k], u.ageTable[k]/3600)
	}

	for k, v := range u.cronTable {
		if int(u.ageTable[k]/3600) > v {
			log.Println(k+" needs updating:", u.ageTable[k]/3600)
		}
	}
}

func (u *AutoUpdater) UpdateEntry(name string, interval int) {
	u.cronTable[name] = interval
}

func (u *AutoUpdater) GetEntries() map[string]int {
	return u.cronTable
}

func (u *AutoUpdater) DelEntry(name string) {
	delete(u.cronTable, name)
	delete(u.ageTable, name)
}
