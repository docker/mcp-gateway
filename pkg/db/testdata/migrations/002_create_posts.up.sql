create table posts (
  id integer primary key,
  user_id integer not null,
  title text not null,
  content text,
  foreign key (user_id) references users(id)
);
