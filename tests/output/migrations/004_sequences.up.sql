SET SESSION statement_timeout = 3000;
SET SESSION lock_timeout = 3000;

/* Hazards:
 - HAS_UNTRACKABLE_DEPENDENCIES: This sequence has no owner, so it cannot be tracked. It may be in use by a table or function.
*/
CREATE SEQUENCE "factory"."seq_machines_id"
	AS bigint
	INCREMENT BY 1
	MINVALUE 0 MAXVALUE 2147483647
	START WITH 1 CACHE 1 NO CYCLE
;

/* Hazards:
 - HAS_UNTRACKABLE_DEPENDENCIES: This sequence has no owner, so it cannot be tracked. It may be in use by a table or function.
*/
CREATE SEQUENCE "warehouse"."seq_storage_locations_id"
	AS bigint
	INCREMENT BY 1
	MINVALUE 0 MAXVALUE 2147483647
	START WITH 1 CACHE 1 NO CYCLE
;

ALTER TABLE "factory"."machines" ADD COLUMN "id" bigint DEFAULT nextval('factory.seq_machines_id'::regclass) NOT NULL;

/* Hazards:
 - ACQUIRES_SHARE_LOCK: Non-concurrent index creates will lock out writes to the table during the duration of the index build.
*/
CREATE UNIQUE INDEX machines_pk ON factory.machines USING btree (id);

ALTER TABLE "factory"."machines" ADD CONSTRAINT "machines_pk" PRIMARY KEY USING INDEX "machines_pk";

ALTER TABLE "warehouse"."storage_locations" ADD COLUMN "id" bigint DEFAULT nextval('warehouse.seq_storage_locations_id'::regclass) NOT NULL;

/* Hazards:
 - ACQUIRES_SHARE_LOCK: Non-concurrent index creates will lock out writes to the table during the duration of the index build.
*/
CREATE UNIQUE INDEX storage_locations_pk ON warehouse.storage_locations USING btree (id);

ALTER TABLE "warehouse"."storage_locations" ADD CONSTRAINT "storage_locations_pk" PRIMARY KEY USING INDEX "storage_locations_pk";

