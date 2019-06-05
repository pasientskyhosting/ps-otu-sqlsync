package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	// mysql driver
	_ "github.com/go-sql-driver/mysql"
)

// DB ...
type DB struct {
	*sql.DB
}

// OTU ...
type OTU struct {
	Host       string
	User       string
	ExpireTime int64
}

var createTableStatements = []string{
	`CREATE DATABASE IF NOT EXISTS ps_otu_sql DEFAULT CHARACTER SET = 'utf8' DEFAULT COLLATE 'utf8_general_ci';`,
	`USE ps_otu_sql;`,
	`CREATE TABLE IF NOT EXISTS user (
		Host char(60) COLLATE utf8_bin NOT NULL DEFAULT '',
		User char(80) COLLATE utf8_bin NOT NULL DEFAULT '',
		Expire_time int(11) NOT NULL DEFAULT 0,
		PRIMARY KEY (Host, User)
	)`,
}

// NewDB returns a new database connection
func NewDB(dataSourceName string) (*DB, error) {
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	if err = db.Ping(); err != nil {
		return nil, err
	}
	return &DB{db}, nil
}

// CreateTable creates the user table, and if necessary, the otu database.
func CreateTable(db *DB) error {
	for _, stmt := range createTableStatements {
		_, err := db.Exec(stmt)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateUser creates a new one-time-user
func CreateUser(db *DB, host string, user string, password string, expireTime int64) error {
	// begin transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec("USE ps_otu_sql")
	if err != nil {
		return err
	}
	// insert otu record
	_, err = tx.Exec("INSERT IGNORE INTO user(User, Host, Expire_time) VALUES(?,?,?)", user, host, expireTime)
	if err != nil {
		tx.Rollback()
		return err
	}
	// commit transaction
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}
	/*
		create mysql user
		mysql.user is MyISAM - MyISAM does NOT support transactions.
		We will only create the mysql user if the otu user record has been created
		Parameterized statements can not be used with CREATE, DROP, GRANT etc.
	*/
	//_, err = db.Exec("INSERT IGNORE INTO mysql.user (Host, User, Password, ssl_cipher, x509_issuer, x509_subject, authentication_string) VALUES(?,?,PASSWORD(?),'','','','')", host, user, password)
	_, err = db.Exec(fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s'", user, host, password))
	if err != nil {
		return err
	}
	return nil
}

// GrantPermissions ...
func GrantPermissions(db *DB, privilege string, level string, user string, host string) error {
	_, err := db.Exec("USE ps_otu_sql")
	if err != nil {
		return err
	}
	/*
		Parameterized statements can not be used with CREATE, DROP, GRANT etc.
	*/
	_, err = db.Exec(fmt.Sprintf("GRANT %s ON %s TO '%s'@'%s'", privilege, level, user, host))
	if err != nil {
		return err
	}
	_, err = db.Exec("FLUSH PRIVILEGES")
	if err != nil {
		return err
	}
	return nil
}

// GetExpiredUsers ...
func GetExpiredUsers(db *DB) ([]*OTU, error) {
	_, err := db.Exec("USE ps_otu_sql")
	if err != nil {
		return nil, err
	}
	rows, err := db.Query("SELECT User, Host, Expire_time FROM user WHERE Expire_time  <= ?", time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	otus := make([]*OTU, 0)
	for rows.Next() {
		otu := new(OTU)
		err := rows.Scan(&otu.User, &otu.Host, &otu.ExpireTime)
		if err != nil {
			return nil, err
		}
		otus = append(otus, otu)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return otus, nil
}

// ExpireUser marks OTU users as expired so they will be dropped
func ExpireUser(db *DB, u []interface{}) error {
	// begin transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec("USE ps_otu_sql")
	if err != nil {
		return err
	}
	// update otu record
	if len(u) > 0 {
		_, err = tx.Exec("UPDATE user SET Expire_time = 0 WHERE User NOT IN (?"+strings.Repeat(",?", len(u)-1)+")", u...)
	} else {
		_, err = tx.Exec("UPDATE user SET Expire_time = 0 WHERE 1=1")
	}
	if err != nil {
		tx.Rollback()
		return err
	}
	// commit transaction
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

// DropUser drops one-time-users
func DropUser(db *DB, u *OTU) error {
	/*
		drop mysql user
		Parameterized statements can not be used with CREATE, DROP, GRANT etc.
	*/
	_, err := db.Exec(fmt.Sprintf("DROP USER IF EXISTS '%s'@'%s'", u.User, u.Host))
	if err != nil {
		return err
	}
	// begin transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec("USE ps_otu_sql")
	if err != nil {
		return err
	}
	// delete otu record
	_, err = tx.Exec("DELETE FROM user WHERE User = ? AND Host = ?", u.User, u.Host)
	if err != nil {
		tx.Rollback()
		return err
	}
	// Commit transaction
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

// set up the otu database
func prepareDatabase(e *Env) (*DB, error) {
	db, err := NewDB(fmt.Sprintf("%s:%s@tcp(%s:%s)/?timeout=5s", e.dbUser, e.dbPassword, e.dbServer, e.dbPort))
	if err != nil {
		return nil, err
	}
	err = CreateTable(db)
	if err != nil {
		db.Close()
		return nil, err
	}
	log.Printf("Successfully prepared ps_otu_sql database...")
	return db, err
}
