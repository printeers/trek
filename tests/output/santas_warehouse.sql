-- ** Database generated with pgModeler (PostgreSQL Database Modeler).
-- ** pgModeler version: 1.2.2
-- ** PostgreSQL version: 18.0
-- ** Project Site: pgmodeler.io
-- ** Model Author: ---

SET check_function_bodies = false;
-- ddl-end --

-- object: warehouse | type: SCHEMA --
-- DROP SCHEMA IF EXISTS warehouse CASCADE;
CREATE SCHEMA warehouse;
-- ddl-end --
ALTER SCHEMA warehouse OWNER TO postgres;
-- ddl-end --

-- object: factory | type: SCHEMA --
-- DROP SCHEMA IF EXISTS factory CASCADE;
CREATE SCHEMA factory;
-- ddl-end --
ALTER SCHEMA factory OWNER TO postgres;
-- ddl-end --

SET search_path TO pg_catalog,public,warehouse,factory;
-- ddl-end --

-- object: warehouse.seq_storage_locations_id | type: SEQUENCE --
-- DROP SEQUENCE IF EXISTS warehouse.seq_storage_locations_id CASCADE;
CREATE SEQUENCE warehouse.seq_storage_locations_id
	INCREMENT BY 1
	MINVALUE 0
	MAXVALUE 2147483647
	START WITH 1
	CACHE 1
	NO CYCLE
	OWNED BY NONE;

-- ddl-end --
ALTER SEQUENCE warehouse.seq_storage_locations_id OWNER TO postgres;
-- ddl-end --

-- object: factory.seq_machines_id | type: SEQUENCE --
-- DROP SEQUENCE IF EXISTS factory.seq_machines_id CASCADE;
CREATE SEQUENCE factory.seq_machines_id
	INCREMENT BY 1
	MINVALUE 0
	MAXVALUE 2147483647
	START WITH 1
	CACHE 1
	NO CYCLE
	OWNED BY NONE;

-- ddl-end --
ALTER SEQUENCE factory.seq_machines_id OWNER TO postgres;
-- ddl-end --

-- object: factory.machines | type: TABLE --
-- DROP TABLE IF EXISTS factory.machines CASCADE;
CREATE TABLE factory.machines (
	id bigint NOT NULL DEFAULT nextval('factory.seq_machines_id'::regclass),
	name text NOT NULL,
	toys_produced bigint NOT NULL,
	CONSTRAINT machines_pk PRIMARY KEY (id)
);
-- ddl-end --
ALTER TABLE factory.machines OWNER TO postgres;
-- ddl-end --

-- object: warehouse.storage_locations | type: TABLE --
-- DROP TABLE IF EXISTS warehouse.storage_locations CASCADE;
CREATE TABLE warehouse.storage_locations (
	id bigint NOT NULL DEFAULT nextval('warehouse.seq_storage_locations_id'::regclass),
	shelf bigint NOT NULL,
	total_capacity bigint NOT NULL,
	used_capacity bigint NOT NULL,
	current_toy_type text NOT NULL,
	CONSTRAINT storage_locations_pk PRIMARY KEY (id),
	CONSTRAINT ck_capacity CHECK (total_capacity >= used_capacity)
);
-- ddl-end --
ALTER TABLE warehouse.storage_locations OWNER TO postgres;
-- ddl-end --

-- object: factory.tr_machines_toys_produced_increase | type: FUNCTION --
-- DROP FUNCTION IF EXISTS factory.tr_machines_toys_produced_increase() CASCADE;
CREATE OR REPLACE FUNCTION factory.tr_machines_toys_produced_increase ()
	RETURNS trigger
	LANGUAGE plpgsql
	VOLATILE 
	CALLED ON NULL INPUT
	SECURITY INVOKER
	PARALLEL UNSAFE
	COST 1
	AS 
$function$
BEGIN
	IF NEW.toys_produced < OLD.toys_produced THEN
		RAISE EXCEPTION 'Toys produced count can not be lowered';
	END IF;
END;
$function$;
-- ddl-end --
ALTER FUNCTION factory.tr_machines_toys_produced_increase() OWNER TO postgres;
-- ddl-end --

-- object: toys_produced_increase | type: TRIGGER --
-- DROP TRIGGER IF EXISTS toys_produced_increase ON factory.machines CASCADE;
CREATE OR REPLACE TRIGGER toys_produced_increase
	BEFORE UPDATE OF toys_produced
	ON factory.machines
	FOR EACH ROW
	EXECUTE PROCEDURE factory.tr_machines_toys_produced_increase();
-- ddl-end --


