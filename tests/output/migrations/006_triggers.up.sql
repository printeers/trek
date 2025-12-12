SET SESSION statement_timeout = 3000;
SET SESSION lock_timeout = 3000;

/* Hazards:
 - HAS_UNTRACKABLE_DEPENDENCIES: Dependencies, i.e. other functions used in the function body, of non-sql functions cannot be tracked. As a result, we cannot guarantee that function dependencies are ordered properly relative to this statement. For adds, this means you need to ensure that all functions this function depends on are created/altered before this statement.
*/
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

