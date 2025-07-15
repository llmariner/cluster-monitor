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

export type RequestFilter = {
  start_timestamp?: string
  end_timestamp?: string
}

export type ListClusterSnapshotsRequest = {
  filter?: RequestFilter
  group_by?: ListClusterSnapshotsRequestGroupBy
}

export type ListClusterSnapshotsResponseValue = {
  grouping_value?: string
  node_count?: number
  gpu_capacity?: number
  memory_capacity_gb?: number
  gpu_occupancy?: number
  pod_count?: number
}

export type ListClusterSnapshotsResponseDatapoint = {
  timestamp?: string
  values?: ListClusterSnapshotsResponseValue[]
}

export type ListClusterSnapshotsResponse = {
  datapoints?: ListClusterSnapshotsResponseDatapoint[]
  cluster_count?: number
}

export type ListGpuUsagesRequest = {
  filter?: RequestFilter
}

export type ListGpuUsagesResponseValue = {
  cluster_name?: string
  node_name?: string
  max_gpu_used?: number
  avg_gpu_used?: number
  max_gpu_memory_used_gb?: number
  avg_gpu_memory_used_gb?: number
}

export type ListGpuUsagesResponseDatapoint = {
  timestamp?: string
  values?: ListGpuUsagesResponseValue[]
}

export type ListGpuUsagesResponse = {
  datapoints?: ListGpuUsagesResponseDatapoint[]
}

export class ClusterMonitorService {
  static ListClusterSnapshots(req: ListClusterSnapshotsRequest, initReq?: fm.InitReq): Promise<ListClusterSnapshotsResponse> {
    return fm.fetchReq<ListClusterSnapshotsRequest, ListClusterSnapshotsResponse>(`/v1/clustertelemetry/clustersnapshots?${fm.renderURLSearchParams(req, [])}`, {...initReq, method: "GET"})
  }
  static ListGpuUsages(req: ListGpuUsagesRequest, initReq?: fm.InitReq): Promise<ListGpuUsagesResponse> {
    return fm.fetchReq<ListGpuUsagesRequest, ListGpuUsagesResponse>(`/v1/clustertelemetry/gpu-usages?${fm.renderURLSearchParams(req, [])}`, {...initReq, method: "GET"})
  }
}