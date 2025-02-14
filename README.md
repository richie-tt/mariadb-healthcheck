# mariadb-healthcheck

[![codecov](https://codecov.io/gh/richie-tt/mariadb-healthcheck/branch/master/graph/badge.svg)](https://codecov.io/gh/richie-tt/mariadb-healthcheck)

## Overview

This project provides a sidecar container for **MariaDB** pods in Kubernetes, specifically designed to perform basic commands like INSERT, SELECT, and DELETE on a dedicated database and expose the result as a HTTP endpoint. This allow Kubernetes to restart the **MariaDB** container if the database is not healthy.

### How it works

The `mariadb-healthcheck` container will perform a check on the database by executing in sequence the `INSERT`, `SELECT`, and `DELETE` commands:

```mermaid
flowchart LR
    A[(INSERT)] --> B[(SELECT)]
    B --> C{delete row }
    C -.-> |yes|D[(DELETE)]
    C -.-> |no|E(HTTP Server)
    D --> E
```

On base of the checks results, Kubernetes will restart the **MariaDB** container even though the **MariaDB** container is not exposing the HTTP server on port `8080`.


From bellow diagram you can notice that the `livenessProbe` and `readinessProbe` checks from the **MariaDB** container will be performed on the `mariadb-healthcheck` container HTTP server endpoint, this will allow Kubernetes to restart the **MariaDB** container if the database is not healthy.

![liveness_and_readiness](./assets/liveness_and_readiness.svg)

## Usage

Environment variables:

| Variable    | Required | Default       | Description                                                                                                                                                                                                |
| ----------- | -------- | ------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| DELETE_ROW  | No       | `true`        | After executing `INSERT` and `SELECT` commands, `DELETE` command only deletes the created row in the table. For debugging purposes, you can disable the delete process by setting this variable to `false` |
| DB_HOST     | No       | `127.0.0.1`   | The host of the database                                                                                                                                                                                   |
| DB_NAME     | No       | `healthcheck` | The name of the database                                                                                                                                                                                   |
| DB_PASSWORD | No       | `healthcheck` | The password of the database                                                                                                                                                                               |
| DB_PORT     | No       | `3306`        | The port of the database                                                                                                                                                                                   |
| DB_USER     | No       | `healthcheck` | The user of the database                                                                                                                                                                                   |
| HEALTH_PORT | No       | `8080`        | The port of HTTP server, where status of check is exposed                                                                                                                                                  |
| LOG_LEVEL   | No       | `info`        | The log level, `debug`, `info`, `warn`, `error`                                                                                                                                                            |


## Installation

### Database

Create a database and a user with the following privileges:

```sql
CREATE DATABASE `healthcheck` /*!40100 DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci */;
CREATE USER 'healthcheck'@'127.0.0.1' IDENTIFIED BY 'healthcheck';
GRANT ALL PRIVILEGES ON `healthcheck`.* TO 'healthcheck'@'127.0.0.1';
```

Create a table with specific engine and charset to store the status of the database:

- `ENGINE=MEMORY` will perform the check on a database that is stored in memory and all data for the check will be lost when the container is restarted, this is not a problem for `mariadb-healthcheck` because it just executes simple commands like `INSERT`, `SELECT`, and `DELETE` on the database. This engine can be a good choice if you want to check the status of the database without impacting its performance.

  > [!WARNING]
  >  You need to be aware that `MEMORY` engine will not check if the Kubernetes `volume` is configured correctly to preform the write operation.

- `ENGINE=ARIA` will perform a check on the database which stores the result of an operation on the disk,  which will allow you to get a better overview of the database status. This engine can have a negative impact on the performance of the database, where a huge load is expected.

```sql
CREATE TABLE healthcheck.status (
	uuid varchar(50) NOT NULL
)
ENGINE=ARIA
DEFAULT CHARSET=utf8mb4
COLLATE=utf8mb4_unicode_ci;
```

### Deployment

Add the sidecar container to your MariaDB pod definition:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mariadb
  labels:
    app: mariadb
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: mariadb
  template:
    metadata:
      labels:
        app: mariadb
    spec:
      containers:
        # This container is used to check the health of the database
        - name: healthcheck
          image: richiett/mariadb-healthcheck:latest
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
              scheme: HTTP
            failureThreshold: 4
            initialDelaySeconds: 15
            periodSeconds: 15
            successThreshold: 1
            timeoutSeconds: 5
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
              scheme: HTTP
            failureThreshold: 1
            initialDelaySeconds: 10
            periodSeconds: 15
            successThreshold: 1
            timeoutSeconds: 5
          ports:
            - name: healthcheck
              containerPort: 8080

        # This container is the MariaDB container
        - name: mariadb
          image: mariadb
          ports:
            - name: mariadb
              containerPort: 3306

          # It's important to add a readinessProbe and livenessProbe to the MariaDB container, even though MariaDB does not expose the 8080 port.
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
              scheme: HTTP
            failureThreshold: 4
            initialDelaySeconds: 15
            periodSeconds: 15
            successThreshold: 1
            timeoutSeconds: 5
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
              scheme: HTTP
            failureThreshold: 1
            initialDelaySeconds: 10
            periodSeconds: 15
            successThreshold: 1
            timeoutSeconds: 5
```
