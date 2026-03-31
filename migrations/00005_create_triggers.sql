-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION audit_log_immutable()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_log records are immutable';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trg_audit_log_immutable
BEFORE UPDATE OR DELETE ON audit_log
FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION check_double_entry()
RETURNS TRIGGER AS $$
DECLARE
    total_debits  BIGINT;
    total_credits BIGINT;
BEGIN
    SELECT
        COALESCE(SUM(CASE WHEN direction = 'DEBIT'  THEN amount ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN direction = 'CREDIT' THEN amount ELSE 0 END), 0)
    INTO total_debits, total_credits
    FROM entries
    WHERE transaction_id = NEW.transaction_id;

    IF total_debits <> total_credits THEN
        RAISE EXCEPTION 'double-entry invariant violated: debits=% credits=%',
            total_debits, total_credits;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE CONSTRAINT TRIGGER trg_check_double_entry
AFTER INSERT ON entries
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION check_double_entry();
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_check_double_entry ON entries;
-- +goose StatementEnd

-- +goose StatementBegin
DROP FUNCTION IF EXISTS check_double_entry;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_audit_log_immutable ON audit_log;
-- +goose StatementEnd

-- +goose StatementBegin
DROP FUNCTION IF EXISTS audit_log_immutable;
-- +goose StatementEnd
