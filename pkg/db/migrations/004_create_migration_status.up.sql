create table migration_status (
  id integer primary key,
  status text not null,
  logs text,
  last_updated DATETIME DEFAULT CURRENT_TIMESTAMP
);
