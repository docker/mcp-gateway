-- Add support for remote server type
-- SQLite doesn't support modifying CHECK constraints, so we need to recreate the table

CREATE TABLE catalog_server_new (
  id integer primary key,
  server_type text check(server_type in ('registry', 'image', 'remote')),
  tools text CHECK (json_valid(tools)),
  source text,
  image text,
  endpoint text,
  snapshot text CHECK (json_valid(snapshot)),
  catalog_ref text not null,
  foreign key (catalog_ref) references catalog(ref) on delete cascade
);

-- Copy existing data
INSERT INTO catalog_server_new (id, server_type, tools, source, image, endpoint, snapshot, catalog_ref)
SELECT id, server_type, tools, source, image, "", snapshot, catalog_ref
FROM catalog_server;

DROP TABLE catalog_server;

ALTER TABLE catalog_server_new RENAME TO catalog_server;
