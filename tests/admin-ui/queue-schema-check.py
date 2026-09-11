"""Independent structural-schema checks; requires jsonschema 4.x, no network."""
import json
from pathlib import Path
from jsonschema import Draft202012Validator

root = Path(__file__).resolve().parents[2]
schema = json.loads((root / "internal/topology/queue-schema.json").read_text(encoding="utf-8"))
cases = json.loads((root / "internal/topology/testdata/queue-schema-cases.json").read_text(encoding="utf-8"))
Draft202012Validator.check_schema(schema)
validator = Draft202012Validator(schema)
for case in cases:
    errors = list(validator.iter_errors(case["document"]))
    assert (not errors) == case["schemaValid"], (case["name"], [error.message for error in errors])
print(f"Schema metaschema and {len(cases)} structural cases passed; custom formats remain server-validated.")
