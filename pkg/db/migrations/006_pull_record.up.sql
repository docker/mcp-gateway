create table pull_record (
  id integer primary key,
  ref text not null unique,
  first_pulled datetime default current_timestamp
);
