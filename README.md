# mariadb-healthcheck

This is a **sidecar** for MariaDB that allows you to monitor the status of your database in a kubernetes environment.

## Overview

This project provides a sidecar container for **MariaDB** pods in Kubernetes, specifically designed to perform basic commands like `INSERT`, `SELECT` and `DELETE` on dedicated database.

## Installation

### Database

Create a database and a user with the following privileges:

```sql
CREATE DATABASE `healthcheck` /*!40100 DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci */;
CREATE USER 'healthcheck'@'127.0.0.1' IDENTIFIED BY 'healthcheck';
GRANT ALL PRIVILEGES ON `healthcheck`.* TO 'healthcheck'@'127.0.0.1';
```

Create a table with engine `ARIA` to store the status of the database:

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
          # If the healthcheck returns a status other than 200, Kubernetes will restart ALL containers that were configured with the same readinessProbe and livenessProbe and finally MariaDB will be restarted too.
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
