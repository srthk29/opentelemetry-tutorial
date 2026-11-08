package instrumented

import (
	"context"
	"fmt"
	"net/http"

	"github.com/sarthak/opentelemetry-tutorial/internal/instrumented/codegen"
)

type openAIEmbeddingClient struct {
	client         codegen.ClientWithResponsesInterface
	model          string
	dimensionality *int32
	isNormalized   bool
}

func NewOpenAIEmbeddingClient(ctx context.Context, model string, dimensionality int32, isNormalized bool) (EmbedderWithEmbeddingResponse, error) {
	client, err := codegen.NewClientWithResponses("http://127.0.0.1:8080/")
	if err != nil {
		return nil, err
	}

	return &openAIEmbeddingClient{
		client:         client,
		model:          model,
		dimensionality: &dimensionality,
		isNormalized:   isNormalized,
	}, nil
}

func (gc *openAIEmbeddingClient) GetEmbedding(ctx context.Context, text string) (*codegen.EmbeddingResponse, error) {

	var input codegen.EmbeddingRequest_Input
	if err := input.FromEmbeddingRequestInput0(text); err != nil {
		return nil, err
	}

	var model codegen.EmbeddingRequest_Model
	if err := model.FromEmbeddingRequestModel0(gc.model); err != nil {
		return nil, err
	}

	var dims *int
	if gc.dimensionality != nil {
		d := int(*gc.dimensionality)
		dims = &d
	}

	resp, err := gc.client.CreateEmbeddingWithResponse(ctx, codegen.EmbeddingRequest{
		Dimensions:     dims,
		EncodingFormat: new(codegen.Float),
		Input:          input,
		Model:          model,
	})
	if err != nil {
		return nil, err
	}

	if resp.HTTPResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s\t%s", resp.Status(), string(resp.Body))
	}

	return resp.JSON200, nil
}

func (gc *openAIEmbeddingClient) GetEmbeddings(ctx context.Context, texts []string) (*codegen.EmbeddingResponse, error) {
	var input codegen.EmbeddingRequest_Input
	if err := input.FromEmbeddingRequestInput1(texts); err != nil {
		return nil, err
	}

	var model codegen.EmbeddingRequest_Model
	if err := model.FromEmbeddingRequestModel0(gc.model); err != nil {
		return nil, err
	}

	var dims *int
	if gc.dimensionality != nil {
		d := int(*gc.dimensionality)
		dims = &d
	}

	resp, err := gc.client.CreateEmbeddingWithResponse(ctx, codegen.EmbeddingRequest{
		Dimensions:     dims,
		EncodingFormat: new(codegen.Float),
		Input:          input,
		Model:          model,
	})
	if err != nil {
		return nil, err
	}

	if resp.HTTPResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s\t%s", resp.Status(), string(resp.Body))
	}

	return resp.JSON200, nil
}

func (gc *openAIEmbeddingClient) GetModel() string { return gc.model }

func (gc *openAIEmbeddingClient) GetDimensionality() int {
	if gc.dimensionality == nil {
		return 0
	}
	return int(*gc.dimensionality)
}

func (gc *openAIEmbeddingClient) GetQueryEmbedding(ctx context.Context, text string, taskType TaskType) (*codegen.EmbeddingResponse, error) {
	return gc.GetEmbedding(ctx, PrepareQuery(taskType, text))
}

func (gc *openAIEmbeddingClient) GetDocumentEmbedding(ctx context.Context, content *Document) (*codegen.EmbeddingResponse, error) {
	return gc.GetEmbedding(ctx, PrepareDocument(content.Title, content.Content))
}

// title: {title} | text: {content}
func (gc *openAIEmbeddingClient) GetDocumentEmbeddings(ctx context.Context, contents []*Document) (*codegen.EmbeddingResponse, error) {
	texts := make([]string, 0, len(contents))
	for _, content := range contents {
		texts = append(texts, PrepareDocument(content.Title, content.Content))
	}

	return gc.GetEmbeddings(ctx, texts)
}

func (gc *openAIEmbeddingClient) IsNormalized() bool { return gc.isNormalized }
