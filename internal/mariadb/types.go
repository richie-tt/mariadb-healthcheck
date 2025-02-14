package mariadb

// Query is a struct that contains required information to execute a query
type Query struct {
	Value string
}

// Connection is a struct that contains required information to connect to a database
type Connection struct {
	Driver   string `env:"DB_DRIVER"`
	Database string `env:"DB_DATABASE"`
	Host     string `env:"DB_HOST"`
	Password string `env:"DB_PASSWORD"`
	Port     string `env:"DB_PORT"`
	User     string `env:"DB_USER"`
}
