# Generated-compatible gRPC helpers for result_analysis.proto.

import grpc

from pb import result_analysis_pb2 as result__analysis__pb2


class ResultAnalysisServiceStub(object):
    def __init__(self, channel):
        self.AnalyzeResultSets = channel.unary_unary(
            "/ling_shu.exec.v1.ResultAnalysisService/AnalyzeResultSets",
            request_serializer=result__analysis__pb2.AnalyzeResultSetsRequest.SerializeToString,
            response_deserializer=result__analysis__pb2.AnalyzeResultSetsResponse.FromString,
        )
        self.Health = channel.unary_unary(
            "/ling_shu.exec.v1.ResultAnalysisService/Health",
            request_serializer=result__analysis__pb2.HealthRequest.SerializeToString,
            response_deserializer=result__analysis__pb2.HealthResponse.FromString,
        )


class ResultAnalysisServiceServicer(object):
    def AnalyzeResultSets(self, request, context):
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Method not implemented!")
        raise NotImplementedError("Method not implemented!")

    def Health(self, request, context):
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Method not implemented!")
        raise NotImplementedError("Method not implemented!")


def add_ResultAnalysisServiceServicer_to_server(servicer, server):
    rpc_method_handlers = {
        "AnalyzeResultSets": grpc.unary_unary_rpc_method_handler(
            servicer.AnalyzeResultSets,
            request_deserializer=result__analysis__pb2.AnalyzeResultSetsRequest.FromString,
            response_serializer=result__analysis__pb2.AnalyzeResultSetsResponse.SerializeToString,
        ),
        "Health": grpc.unary_unary_rpc_method_handler(
            servicer.Health,
            request_deserializer=result__analysis__pb2.HealthRequest.FromString,
            response_serializer=result__analysis__pb2.HealthResponse.SerializeToString,
        ),
    }
    generic_handler = grpc.method_handlers_generic_handler(
        "ling_shu.exec.v1.ResultAnalysisService", rpc_method_handlers
    )
    server.add_generic_rpc_handlers((generic_handler,))
