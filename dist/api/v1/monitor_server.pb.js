/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/
import * as fm from "../../fetch.pb";
export var ListClusterSnapshotsRequestGroupBy;
(function (ListClusterSnapshotsRequestGroupBy) {
    ListClusterSnapshotsRequestGroupBy["GROUP_BY_UNSPECIFIED"] = "GROUP_BY_UNSPECIFIED";
    ListClusterSnapshotsRequestGroupBy["GROUP_BY_CLUSTER"] = "GROUP_BY_CLUSTER";
    ListClusterSnapshotsRequestGroupBy["GROUP_BY_PRODUCT"] = "GROUP_BY_PRODUCT";
})(ListClusterSnapshotsRequestGroupBy || (ListClusterSnapshotsRequestGroupBy = {}));
export class ClusterMonitorService {
    static ListClusterSnapshots(req, initReq) {
        return fm.fetchReq(`/v1/clustertelemetry/clustersnapshots?${fm.renderURLSearchParams(req, [])}`, Object.assign(Object.assign({}, initReq), { method: "GET" }));
    }
    static ListGpuUsages(req, initReq) {
        return fm.fetchReq(`/v1/clustertelemetry/gpu-usages?${fm.renderURLSearchParams(req, [])}`, Object.assign(Object.assign({}, initReq), { method: "GET" }));
    }
}
