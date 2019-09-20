# PatientSky One Time User - MySQL Sync Service

## Description
Self-service access to MySQL/MariaDB using LDAP credentials

MySQL Sync Service synchronizes one-time-users created in the One Time User service https://github.com/pasientskyhosting/ps-otu-ldap

### One Time User Service - Custom props
The following custom props should be set on your OTU group to configure host and privileges for OTU accounts

Key `priv_type` - Privilege types for the GRANT statement

Example values
- `ALL`
- `SELECT`
- `INSERT`
- `CREATE`
- `UPDATE`
- `SELECT,INSERT`
- etc.

Key `priv_level` - Privilege level

Example values
- `*`
- `*.*`
- `db_name.*`
- `db_name.tbl_name`
- `tbl_name`
- `db_name.routine_name`

Key `host` 

Example values
- `%`
- `localhost`
- `127.0.0.1`
- `198.51.100.%`
- `198.51.100.0/255.255.255.0`

## Quickstart

### Step 1 - Setup env
You need these environment variables:
- `DB_USER` - Database user account used to create/delete users and grant privileges
- `DB_PASSWORD` - Database password
- `DB_PORT` - MySQL or MariaDB server port (default: 3306)
- `API_URL` - OTU service API url
- `API_KEY` - OTU service API key
- `LDAP_GROUPS` - Search for OTU bound to these LDAP groups (comma separated list)
- `CLEANUP_INTERVAL` - How often (in seconds) to purge expired/retired users (default: 60) 
- `POLL_INTERVAL` - How often (in seconds) to poll the OTU service (default: 60)
- `METRICS_PORT` - Prometheus metrics port (default: 9597)

```
export DB_USER=dbAccountWithUserManagementPrivs && \
export DB_PASSWORD=someStrongPassword && \
export DB_SERVER=127.0.0.1 && \
export DB_PORT=3306 && \
export API_URL=https://my.otu.service/api/v1 && \
export API_KEY=kjsdfJ79hY73eKh37Hedk98234Ghwhjd823kHY2kHY2 && \
export LDAP_GROUPS=galera-dev-read,galera-dev-full && \
export CLEANUP_INTERVAL=60 && \
export POLL_INTERVAL=60 && \
export METRICS_PORT=9597
```

### Step 2 - Build docker image and run

`make all` to build binaries and create the docker image

`make docker-run` to run the image

You will see similar output as users are synchronized:

```
2019/06/01 20:35:06 otu-sqlsync service started with env: {...}

2019/06/01 20:35:06 Successfully prepared ps_otu_sql database...
2019/06/01 20:35:17 Created user 'cj-elastic-custom-bOhpntF5'@'%': Expires: 2019-06-05 20:41:17 +0200 CEST
2019/06/01 20:35:17 Created user 'cj-elastic-custom-1-t6kl0wwo'@'%': Expires: 2019-06-05 20:41:20 +0200 CEST
2019/06/01 20:36:06 Dropped user: 'cj12354674657899999'@'%'
```

## Makefile
A makefile exists that will help with the following commands:

### Run
Compile and run with `make run`

### Build
Create binaries, upx pack and buld Docker image with `make all`

### Docker Run
Run docker image with `make docker-run`

### Docker Push
Push image to Docker hub with `make docker-push`
