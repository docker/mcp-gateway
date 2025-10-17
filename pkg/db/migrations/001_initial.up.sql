create table working_set (
  id text primary key,
  name text not null,
  servers text not null CHECK (json_valid(servers)),
  secrets text not null CHECK (json_valid(secrets)) 
);