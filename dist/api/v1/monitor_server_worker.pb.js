/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/
import * as fm from "../../fetch.pb";
export class ClusterMonitorWorkerService {
    static SendClusterTelemetry(req, initReq) {
        return fm.fetchReq(`/llmariner.clustermonitor.server.v1.ClusterMonitorWorkerService/SendClusterTelemetry`, Object.assign(Object.assign({}, initReq), { method: "POST", body: JSON.stringify(req) }));
    }
}
