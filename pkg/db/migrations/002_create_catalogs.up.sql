create table catalog (
  id integer primary key,
  digest text not null unique,
  name text not null,
  source text
);

create table catalog_server (
  id integer primary key,
  server_type text check(server_type in ('registry', 'image')),
  tools text CHECK (json_valid(tools)),
  source text,
  image text,
  snapshot text CHECK (json_valid(snapshot)),
  catalog_id integer not null,
  foreign key (catalog_id) references catalog(id) on delete cascade
);

