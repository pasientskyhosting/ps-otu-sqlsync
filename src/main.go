package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Env ...
type Env struct {
	dbUser          string
	dbPassword      string
	dbServer        string
	dbPort          string
	apiURL          string
	apiKey          string
	ldapGroup       string
	cleanupInterval int
	pollInterval    int
	metricsPort     string
}

type dbUser struct {
	username   string
	password   string
	host       string
	privType   string
	privLevel  string
	expireTime int64
}

type single struct {
	mu   sync.Mutex
	user map[string]int64
}

var (
	dbStatus = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "ps_otu_sqlsync_db_status",
			Help: "Database connection status",
		})
	usersCreated = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "ps_otu_sqlsync_users_created_total",
			Help: "The total number of users created",
		})
	usersDropped = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "ps_otu_sqlsync_users_dropped_total",
			Help: "The total number of users dropped",
		})
	cache = single{
		user: make(map[string]int64),
	}
)

// Exists checks for value existing
func (s *single) Exists(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, e := s.user[key]
	return e
}

// Get a key (if exists)
func (s *single) Get(key string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.user[key]
}

// Set a key
func (s *single) Set(key string, v int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.user[key] = v
}

// Delete a key
func (s *single) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.user, key)
}

// All keys and their value
func (s *single) All(key string) (m map[string]int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range s.user {
		m[i] = v
	}
	return m
}

func newEnv(
	dbUser string,
	dbPassword string,
	dbServer string,
	dbPort string,
	apiURL string,
	apiKey string,
	ldapGroup string,
	cleanupInterval int,
	pollInterval int,
	metricsPort string) *Env {

	if dbUser == "" {
		log.Fatalf("Could not parse env DB_USER")
	}
	if dbPassword == "" {
		log.Fatalf("Could not parse env DB_PASSWORD")
	}
	if dbServer == "" {
		log.Fatalf("Could not parse env DB_SERVER")
	}
	if dbPort == "" {
		dbPort = "3306"
	}
	if apiURL == "" {
		log.Fatalf("Could not parse env API_URL")
	}
	if apiKey == "" {
		log.Fatalf("Could not parse env API_KEY")
	}
	if apiKey == "" {
		log.Fatalf("Could not parse env LDAP_GROUP")
	}
	if cleanupInterval == 0 {
		cleanupInterval = 60
	}
	if pollInterval == 0 {
		pollInterval = 60
	}
	if metricsPort == "" {
		metricsPort = "9597"
	}

	e := Env{
		dbUser:          dbUser,
		dbPassword:      dbPassword,
		dbServer:        dbServer,
		dbPort:          dbPort,
		apiURL:          apiURL,
		apiKey:          apiKey,
		ldapGroup:       ldapGroup,
		cleanupInterval: cleanupInterval,
		pollInterval:    pollInterval,
		metricsPort:     metricsPort,
	}

	log.Printf("otu-sqlsync service started with env: %+v\n\n", e)

	return &e
}

// set Custom Properties defaults
func setDefaultCustomProps(m map[string]string) map[string]string {
	_, e := m["priv_type"]
	if !e {
		m["priv_type"] = "SELECT"
	}
	_, e = m["priv_level"]
	if !e {
		m["priv_level"] = "*.*"
	}
	_, e = m["host"]
	if !e {
		m["host"] = "%"
	}
	return m
}

func getOTU(e *Env) ([]dbUser, error) {
	su := []dbUser{}
	group, err := getAPIGroup(e)
	if err != nil {
		return nil, err
	}
	for _, g := range group {
		g.CustomProperties = setDefaultCustomProps(g.CustomProperties)
		user, err := getAPIUser(e, g.GroupName)
		if err != nil {
			return nil, err
		}
		for _, u := range user {
			su = append(su, dbUser{
				username:   u.Username,
				password:   u.Password,
				host:       g.CustomProperties["host"],
				privType:   g.CustomProperties["priv_type"],
				privLevel:  g.CustomProperties["priv_level"],
				expireTime: u.ExpireTime,
			})
		}
	}
	return su, nil
}

func dropOTU(e *Env, db *DB) {
	ticker := time.NewTicker(time.Second * time.Duration(e.cleanupInterval)).C
	for {
		select {
		case <-ticker:
			if err := db.Ping(); err != nil {
				dbStatus.Set(0)
			} else {
				dbStatus.Set(1)
			}
			user, err := GetExpiredUsers(db)
			if err != nil {
				log.Printf("%s", err)
			}
			for _, n := range user {
				err = DropUser(db, n)
				if err != nil {
					log.Printf("%s", err)
					continue
				}
				cache.Delete(n.User)
				usersDropped.Inc()
				log.Printf("Dropped user: '%s'@'%s'\n", n.User, n.Host)
			}
		}
	}
}

func expireOTU(e *Env, db *DB, dbu []dbUser) error {
	u := make([]interface{}, len(dbu))
	for i, v := range dbu {
		u[i] = v.username
	}
	err := ExpireUser(db, u)
	if err != nil {
		return err
	}
	return nil
}

func pollAPI(e *Env, db *DB) {
	ticker := time.NewTicker(time.Second * time.Duration(e.pollInterval)).C
	for {
		select {
		case <-ticker:
			otu, err := getOTU(e)
			if err != nil {
				log.Printf("%s", err)
				continue
			}
			err = expireOTU(e, db, otu)
			if err != nil {
				log.Printf("%s", err)
			}
			for _, u := range otu {
				if !cache.Exists(u.username) {
					err = CreateUser(db, u.host, u.username, u.password, u.expireTime)
					if err != nil {
						log.Printf("%s", err)
						continue
					}
					err = GrantPermissions(db, u.privType, u.privLevel, u.username, u.host)
					if err != nil {
						log.Printf("%s", err)
						continue
					}
					cache.Set(u.username, u.expireTime)
					usersCreated.Inc()
					log.Printf("Created user: '%s'@'%s' Expires: %s", u.username, u.host, time.Unix(u.expireTime, 0))
				}
			}
		}
	}
}

func getenvInt(key string) int {
	s := os.Getenv(key)
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

func mainloop() {
	exitSignal := make(chan os.Signal)
	signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM)
	<-exitSignal

	systemTeardown()
}

func systemTeardown() {
	log.Printf("Shutting down...")
}

func main() {
	var err error
	defer func() {
		if err != nil {
			log.Fatalln(err)
		}
	}()
	// get env
	e := newEnv(os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_SERVER"),
		os.Getenv("DB_PORT"),
		os.Getenv("API_URL"),
		os.Getenv("API_KEY"),
		os.Getenv("LDAP_GROUP"),
		getenvInt("CLEANUP_INTERVAL"),
		getenvInt("POLL_INTERVAL"),
		os.Getenv("METRICS_PORT"),
	)
	// prepare database
	db, err := prepareDatabase(e)
	if err != nil {
		return
	}
	defer db.Close()
	go dropOTU(e, db)
	go pollAPI(e, db)
	// prometheus metrics
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(fmt.Sprintf(":%s", e.metricsPort), nil)
	mainloop()
}
