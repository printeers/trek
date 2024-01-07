alter table "warehouse"."storage_locations" add constraint "ck_capacity" CHECK ((total_capacity >= used_capacity));
