create table users (
  id integer primary key,
  username text not null unique,
  created_at datetime default current_timestamp
);
