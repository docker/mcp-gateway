create table catalog (
  ref text primary key,
  digest text not null,
  title text not null,
  source text
);

create table catalog_server (
  id integer primary key,
  server_type text check(server_type in ('registry', 'image')),
  tools text CHECK (json_valid(tools)),
  source text,
  image text,
  snapshot text CHECK (json_valid(snapshot)),
  catalog_ref text not null,
  foreign key (catalog_ref) references catalog(ref) on delete cascade
);

