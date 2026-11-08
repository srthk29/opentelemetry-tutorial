package instrumented

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/sarthak/opentelemetry-tutorial/internal/instrumented/codegen"
)

type SummarizerWithChatCompletionResponse interface {
	summary(ctx context.Context, msgs []codegen.ChatCompletionRequestMessage) (*codegen.ChatCompletionResponse, error)

	getSummary(ctx context.Context, text string) (*codegen.ChatCompletionResponse, error)
	getLongSummary(ctx context.Context, text string) (*codegen.ChatCompletionResponse, error)
	getLongSummaryWithPromptVersion(ctx context.Context, text string, promptVerison int) (*codegen.ChatCompletionResponse, error)
	getMergedSummary(ctx context.Context, texts []string) (*codegen.ChatCompletionResponse, error)
	getMergedLongSummary(ctx context.Context, texts []string) (*codegen.ChatCompletionResponse, error)
	model() string
}

type openAISummarizer struct {
	client    codegen.ClientWithResponsesInterface
	modelName string
}

func NewOpenAISummarizer(ctx context.Context, modelName string) (SummarizerWithChatCompletionResponse, error) {
	client, err := codegen.NewClientWithResponses("http://127.0.0.1:8080/")
	if err != nil {
		return nil, err
	}
	/*
		./llama-server \
		  -m gemma-4-E4B-it-Q8_0.gguf \
		  -ngl 99 \
		  -fa on \
		  -c 8192 \
		  -b 2048 \
		  -ub 2048 \
		  --cache-type-k q8_0 \
		  --chat-template-kwargs '{"enable_thinking":false}'
	*/
	// https://huggingface.co/google/gemma-4-E4B-it-qat-q4_0-gguf
	// "google/gemma-4-E4B-it-qat-q4_0-gguf"
	return &openAISummarizer{client: client, modelName: modelName}, nil
}

func (gs *openAISummarizer) summary(ctx context.Context, msgs []codegen.ChatCompletionRequestMessage) (*codegen.ChatCompletionResponse, error) {

	resp, err := gs.client.CreateChatCompletionWithResponse(
		ctx,
		codegen.ChatCompletionRequest{
			Model: func() codegen.ModelIdsShared {
				var model codegen.ModelIdsShared
				_ = model.FromModelIdsShared0(gs.modelName)
				return model
			}(),
			Temperature: func() *codegen.ChatCompletionRequest_Temperature {
				var temp codegen.ChatCompletionRequest_Temperature
				_ = temp.FromChatCompletionRequestTemperature0(0.2)
				return &temp
			}(),
			TopP: func() *codegen.ChatCompletionRequest_TopP {
				var topP codegen.ChatCompletionRequest_TopP
				_ = topP.FromChatCompletionRequestTopP0(0.9)
				return &topP
			}(),
			FrequencyPenalty: new(float32(0.0)),
			PresencePenalty:  new(float32(0.0)),
			Stream:           new(false),
			Messages:         msgs,
		})
	if err != nil {
		return nil, err
	}

	if resp.HTTPResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s\t%s", resp.Status(), string(resp.Body))
	}

	return resp.JSON200, nil
}

func (gs *openAISummarizer) getSummary(ctx context.Context, text string) (*codegen.ChatCompletionResponse, error) {
	sysMsg, err := systemMsg(ShortSummarySystemPrompt)
	if err != nil {
		return nil, err
	}

	userMsg, err := userMsg(text)
	if err != nil {
		return nil, err
	}

	return gs.summary(ctx, []codegen.ChatCompletionRequestMessage{*sysMsg, *userMsg})
}

func (gs *openAISummarizer) getLongSummary(ctx context.Context, text string) (*codegen.ChatCompletionResponse, error) {
	sysMsg, err := systemMsg(SummarySystemPrompt)
	if err != nil {
		return nil, err
	}

	userMsg, err := userMsg(text)
	if err != nil {
		return nil, err
	}

	return gs.summary(ctx, []codegen.ChatCompletionRequestMessage{*sysMsg, *userMsg})
}

func (gs *openAISummarizer) getLongSummaryWithPromptVersion(ctx context.Context, text string, promptVerison int) (*codegen.ChatCompletionResponse, error) {

	sysMsg, err := systemMsg(longSummarySystemPromptVersion(
		PromptVersion(promptVerison)))
	if err != nil {
		return nil, err
	}

	userMsg, err := userMsg(text)
	if err != nil {
		return nil, err
	}

	return gs.summary(ctx, []codegen.ChatCompletionRequestMessage{*sysMsg, *userMsg})
}

func (gs *openAISummarizer) getMergedSummary(ctx context.Context, texts []string) (*codegen.ChatCompletionResponse, error) {
	sysMsg, err := systemMsg(MergeSummarySystemPrompt)
	if err != nil {
		return nil, err
	}

	userMsgs, err := userMsgs(texts)
	if err != nil {
		return nil, err
	}

	return gs.summary(ctx, []codegen.ChatCompletionRequestMessage{*sysMsg, *userMsgs})
}

func (gs *openAISummarizer) getMergedLongSummary(ctx context.Context, texts []string) (*codegen.ChatCompletionResponse, error) {
	sysMsg, err := systemMsg(MergeLongSummarySystemPrompt)
	if err != nil {
		return nil, err
	}

	userMsgs, err := userMsgs(texts)
	if err != nil {
		return nil, err
	}

	return gs.summary(ctx, []codegen.ChatCompletionRequestMessage{*sysMsg, *userMsgs})
}

func (gs *openAISummarizer) model() string { return gs.modelName }

func userMsg(text string) (*codegen.ChatCompletionRequestMessage, error) {
	content, err := userMsg_Content(text)
	if err != nil {
		return nil, err
	}

	var msg codegen.ChatCompletionRequestMessage
	if err := msg.FromChatCompletionRequestUserMessage(codegen.ChatCompletionRequestUserMessage{Role: codegen.User, Content: *content}); err != nil {
		return nil, err
	}

	return &msg, nil
}

func userMsgs(texts []string) (*codegen.ChatCompletionRequestMessage, error) {
	var b strings.Builder

	b.WriteString("Technical summaries:\n\n")

	for i, s := range texts {
		fmt.Fprintf(&b, "Summary %d:\n%s\n\n", i+1, s)
	}

	content, err := userMsg_Content(b.String())
	if err != nil {
		return nil, err
	}

	var msg codegen.ChatCompletionRequestMessage
	if err := msg.FromChatCompletionRequestUserMessage(
		codegen.ChatCompletionRequestUserMessage{
			Role:    codegen.User,
			Content: *content,
		},
	); err != nil {
		return nil, err
	}

	return &msg, nil
}

func userMsg_Content(text string) (*codegen.ChatCompletionRequestUserMessage_Content, error) {
	var c codegen.ChatCompletionRequestUserMessage_Content

	if err := c.FromChatCompletionRequestUserMessageContent0(text); err != nil {
		return nil, err
	}

	return &c, nil
}

func systemMsg(text string) (*codegen.ChatCompletionRequestMessage, error) {
	content, err := systemMsg_Content(text)
	if err != nil {
		return nil, err
	}

	var msg codegen.ChatCompletionRequestMessage
	if err := msg.FromChatCompletionRequestSystemMessage(codegen.ChatCompletionRequestSystemMessage{Role: codegen.System, Content: *content}); err != nil {
		return nil, err
	}

	return &msg, nil
}

func systemMsg_Content(text string) (*codegen.ChatCompletionRequestSystemMessage_Content, error) {
	var c codegen.ChatCompletionRequestSystemMessage_Content

	if err := c.FromChatCompletionRequestSystemMessageContent0(text); err != nil {
		return nil, err
	}

	return &c, nil
}

const ShortSummarySystemPrompt string = `
	You are a technical analyst. Extract the core technical value from the provided article into a strict 3-to-5 point bulleted list.

	RULES:
	- Limit to exactly 3 to 5 bullet points.
	- Each bullet must be a single, concise sentence.
	- Retain exact metrics, system architectures, and technical terminology. 
	- Do not simplify the concepts for a layperson.
	- Start outputting bullets immediately with no intro text.
	`

const MergeSummarySystemPrompt string = `
	You are a technical analyst. Merge the provided technical summaries into a single coherent summary that preserves all important information while eliminating redundancy.

	RULES:
	* Limit to exactly 3 to 5 bullet points.
	* Each bullet must be a single, concise sentence.
	* Merge related points instead of repeating them.
	* Preserve exact metrics, system architectures, algorithms, APIs, protocols, benchmarks, and technical terminology.
	* Preserve important trade-offs, limitations, and implementation details.
	* Do not simplify the concepts for a layperson.
	* Do not introduce new information or omit unique technical details present in the summaries.
	* Start outputting bullets immediately with no intro text.
	`

const MergeLongSummarySystemPrompt string = `
	You are a technical analyst. Merge the provided technical summaries into a single coherent summary that preserves all important information while eliminating redundancy.

	<output_format>
	Structure your response exactly using the following markdown headers. If a section is not applicable to the source text, omit it entirely rather than writing "N/A".

	## TL;DR
	Provide a single, hard-hitting sentence summarizing the core breakthrough or main point of the article.

	## Core Problem
	Briefly state the specific technical challenge, bottleneck, or knowledge gap the authors are trying to solve. (1-2 sentences)

	## Methodology & Architecture
	Use bullet points to detail how they solved it. Focus on system design, algorithms used, experimental setups, or novel frameworks.
	* [Key technical detail 1]
	* [Key technical detail 2]

	## Key Results & Metrics
	List the concrete findings. **You must include specific numbers, percentages, and benchmark data if provided in the text.**
	* [Metric/Result 1]
	* [Metric/Result 2]

	## Conclusion & Next Steps
	What are the limitations, implications, or future work mentioned by the authors? (1-2 sentences)
	</output_format>
	`

const SummarySystemPrompt string = `
You are a technical content summarizer. Summarize the provided YouTube transcript accurately, preserving its key concepts, explanations, technical details, examples, and conclusions.

**Rules:**
1. **Strictly source-grounded:** Use only information explicitly stated or clearly explained in the transcript. Do not introduce external knowledge, assumptions, opinions, or invented details. If something is unclear, do not guess.
2. **Preserve technical accuracy:** Retain important terminology, code snippets, commands, formulas, configuration details, numbers, and named technologies. Do not oversimplify technical explanations or change their meaning.
3. **Structure logically:** Start with a descriptive "#" heading. Use "##" and "###" subheadings to organize related topics, concepts, explanations, examples, and conclusions. Include only headings that are relevant to the content.
4. **Prioritize substance:** Capture the main ideas, important supporting details, decisions, trade-offs, limitations, and practical takeaways. Remove repetition, filler, greetings, and irrelevant tangents.
5. **Maintain context:** Preserve relationships between concepts, the order of procedures, and cause-and-effect explanations where relevant. Do not omit details necessary to understand the subject.
6. **Be concise:** Prefer clear, information-dense prose and bullet points. Avoid unnecessary elaboration, redundant statements, and generic introductions or conclusions.
7. **Output format:** Return only the summary as raw Markdown. Do not wrap it in code fences or include preambles, meta-commentary, or information about the summarization process.
	`

const SummarySystemPrompt_Older string = `
	<role>
	You are an Expert Technical Analyst and Senior Engineer. Your task is to distill complex technical articles into highly accurate, structured, and easily scannable summaries. You write for an audience of busy technical professionals who need the core insights without reading the entire paper.
	</role>

	<guidelines>
	1. Fact-Based Strictness: Never invent information, hallucinate data, or extrapolate beyond what is explicitly stated in the text.
	2. Technical Fidelity: Retain crucial technical terms, acronyms, and metrics. Do not over-simplify concepts to the point of losing technical accuracy.
	3. Objective Tone: Maintain a neutral, analytical tone. Do not include your own opinions or praise for the article (e.g., avoid "This is a great article that...").
	4. Signal-to-Noise: Eliminate fluff, rhetorical questions, and marketing speak. Focus strictly on the problem, methodology, and results.
	</guidelines>

	<output_format>
	Structure your response exactly using the following markdown headers. If a section is not applicable to the source text, omit it entirely rather than writing "N/A".

	## TL;DR
	Provide a single, hard-hitting sentence summarizing the core breakthrough or main point of the article.

	## Core Problem
	Briefly state the specific technical challenge, bottleneck, or knowledge gap the authors are trying to solve. (1-2 sentences)

	## Methodology & Architecture
	Use bullet points to detail how they solved it. Focus on system design, algorithms used, experimental setups, or novel frameworks.
	* [Key technical detail 1]
	* [Key technical detail 2]

	## Key Results & Metrics
	List the concrete findings. **You must include specific numbers, percentages, and benchmark data if provided in the text.**
	* [Metric/Result 1]
	* [Metric/Result 2]

	## Conclusion & Next Steps
	What are the limitations, implications, or future work mentioned by the authors? (1-2 sentences)
	</output_format>
	`

type PromptVersion int

const (
	PromptVersion0 PromptVersion = iota
	PromptVersion1
)

func longSummarySystemPromptVersion(promptVersion PromptVersion) string {
	switch promptVersion {
	case PromptVersion0:
		return SummarySystemPrompt_Older
	case PromptVersion1:
		return SummarySystemPrompt
	default:
		return SummarySystemPrompt_Older
	}
}

