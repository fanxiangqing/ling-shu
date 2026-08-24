from __future__ import annotations

from typing import Any, Dict, List

from google.protobuf.struct_pb2 import Struct
from google.protobuf.json_format import MessageToDict


def struct_to_dict(value: Struct) -> Dict[str, Any]:
    return MessageToDict(value, preserving_proto_field_name=True)


def dict_to_struct(value: Dict[str, Any]) -> Struct:
    out = Struct()
    out.update(value or {})
    return out


def rows_from_proto(rows: List[Struct]) -> List[Dict[str, Any]]:
    return [struct_to_dict(row) for row in rows]


def rows_to_proto(rows: List[Dict[str, Any]]) -> List[Struct]:
    return [dict_to_struct(row) for row in rows or []]
