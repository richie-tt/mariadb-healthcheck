# mariadb-healthcheck

This is a **sidecar** for MariaDB that allows you to monitor the status of your database in a kubernetes environment.

[[_TOC_]]

## Overview

This project provides a sidecar container for MariaDB pods in Kubernetes, specifically designed to perform basic commands like `INSERT`, `SELECT` and `DELETE` on dedicated database.

## Installation

### Database

Create a database and a user with the following privileges:

```sql
CREATE DATABASE `healthcheck` /*!40100 DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci */;
CREATE USER 'healthcheck'@'127.0.0.1' IDENTIFIED BY 'healthcheck';
GRANT ALL PRIVILEGES ON `healthcheck`.* TO 'healthcheck'@'127.0.0.1';
```

Create a table with engine `MEMORY` to store the status of the database:
```sql
CREATE TABLE healthcheck.status (
	uuid varchar(50) NOT NULL
)
ENGINE=MEMORY
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

        - name: mariadb
          image: mariadb
          ports:
            - containerPort: 3306

        - name: healthcheck
          image: richiett/mariadb-healthcheck:latest
          env:

            - name: DB_USER
              value: healthcheck
            - name: DB_PASSWORD
              value: healthcheck


```
