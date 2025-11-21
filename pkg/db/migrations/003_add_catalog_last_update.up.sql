alter table catalog add column last_updated DATETIME;
update catalog set last_updated = current_timestamp;
