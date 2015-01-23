package strive

import "time"

type Host struct {
	ID            string
	Name          string
	Resources     map[string]int
	Status        string
	LastHeartbeat time.Time
}
