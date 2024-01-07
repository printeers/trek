create sequence "factory"."seq_machines_id";

create sequence "warehouse"."seq_storage_locations_id";

alter table "factory"."machines" add column "id" bigint not null default nextval('factory.seq_machines_id'::regclass);

alter table "warehouse"."storage_locations" add column "id" bigint not null default nextval('warehouse.seq_storage_locations_id'::regclass);

CREATE UNIQUE INDEX machines_pk ON factory.machines USING btree (id);

CREATE UNIQUE INDEX storage_locations_pk ON warehouse.storage_locations USING btree (id);

alter table "factory"."machines" add constraint "machines_pk" PRIMARY KEY using index "machines_pk";

alter table "warehouse"."storage_locations" add constraint "storage_locations_pk" PRIMARY KEY using index "storage_locations_pk";
