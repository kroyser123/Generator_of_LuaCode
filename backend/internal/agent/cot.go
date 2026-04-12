package agent

// строит промпт с Chain-of-Thought
func BuildCOTPrompt(task string) string {
    return "First, create a step-by-step plan, then generate the code.\n\nTask: " + task + "\n\nPlan:\n1."
}