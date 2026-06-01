package internal

import (
	"context"
	"sort"

	pb "github.com/GoCodeAlone/workflow/plugin/external/proto"
)

var gcpProviderRegions = []string{
	"asia-east1",
	"europe-west1",
	"us-central1", "us-east1", "us-west1",
}

func (s *gcpIaCServer) ListRegions(context.Context, *pb.ListRegionsRequest) (*pb.ListRegionsResponse, error) {
	regions := make([]string, len(gcpProviderRegions))
	copy(regions, gcpProviderRegions)
	sort.Strings(regions)

	out := make([]*pb.ProviderRegion, 0, len(regions))
	for _, name := range regions {
		out = append(out, &pb.ProviderRegion{Name: name, DisplayName: name})
	}
	return &pb.ListRegionsResponse{Regions: out}, nil
}
