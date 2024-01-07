set check_function_bodies = off;

CREATE OR REPLACE FUNCTION factory.tr_machines_toys_produced_increase()
 RETURNS trigger
 LANGUAGE plpgsql
 COST 1
AS $function$
BEGIN
	IF NEW.toys_produced < OLD.toys_produced THEN
		RAISE EXCEPTION 'Toys produced count can not be lowered';
	END IF;
END;
$function$
;

CREATE TRIGGER toys_produced_increase BEFORE UPDATE OF toys_produced ON factory.machines FOR EACH ROW EXECUTE FUNCTION factory.tr_machines_toys_produced_increase();
