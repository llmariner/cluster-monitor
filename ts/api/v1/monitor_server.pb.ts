/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as fm from "../../fetch.pb"

export enum ListClusterSnapshotsRequestGroupBy {
  GROUP_BY_UNSPECIFIED = "GROUP_BY_UNSPECIFIED",
  GROUP_BY_CLUSTER = "GROUP_BY_CLUSTER",
  GROUP_BY_PRODUCT = "GROUP_BY_PRODUCT",
}

export type ListClusterSnapshotsRequestFilter = {
}

export type ListClusterSnapshotsRequest = {
  filter?: ListClusterSnapshotsRequestFilter
  group_by?: ListClusterSnapshotsRequestGroupBy
}

export type ListClusterSnapshotsResponseValue = {
  grouping_value?: string
  node_count?: number
  gpu_capacity?: number
  memory_capacity_gb?: number
}

export type ListClusterSnapshotsResponseDatapoint = {
  timestamp?: string
  values?: ListClusterSnapshotsResponseValue[]
}

export type ListClusterSnapshotsResponse = {
  datapoints?: ListClusterSnapshotsResponseDatapoint[]
}

export class ClusterMonitorService {
  static ListClusterSnapshots(req: ListClusterSnapshotsRequest, initReq?: fm.InitReq): Promise<ListClusterSnapshotsResponse> {
    return fm.fetchReq<ListClusterSnapshotsRequest, ListClusterSnapshotsResponse>(`/v1/clustertelemetry/clustersnapshots?${fm.renderURLSearchParams(req, [])}`, {...initReq, method: "GET"})
  }
}