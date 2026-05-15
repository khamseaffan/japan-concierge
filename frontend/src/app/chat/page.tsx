"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { FormEvent, useMemo, useState } from "react";
import { api } from "@/lib/api";
import type { ConversationMessage, ConversationToolCall } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { Bot, CheckCircle2, MessageCircle, Send, Wrench } from "lucide-react";

type ChatMessage = ConversationMessage & {
  id: string;
  toolCalls?: ConversationToolCall[];
};

const STARTERS = [
  "What tasks are still pending?",
  "I landed in Japan today.",
  "Mark task 1 done.",
] as const;

export default function ChatPage() {
  const queryClient = useQueryClient();
  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      id: "welcome",
      role: "assistant",
      content:
        "Tell me what happened or ask what still needs attention. I can check tasks, log events, and mark tasks done.",
    },
  ]);
  const [draft, setDraft] = useState("");

  const apiMessages = useMemo<ConversationMessage[]>(
    () =>
      messages
        .filter((message) => message.id !== "welcome")
        .map(({ role, content }) => ({ role, content })),
    [messages],
  );

  const sendMessage = useMutation({
    mutationFn: api.sendConversationMessage,
    onSuccess: (response) => {
      setMessages((current) => [
        ...current,
        {
          id: crypto.randomUUID(),
          role: "assistant",
          content:
            response.message ||
            "I completed the tool call, but did not receive a text reply.",
          toolCalls: response.tool_calls,
        },
      ]);
      if (response.tool_calls?.some((call) => call.success)) {
        queryClient.invalidateQueries({ queryKey: ["tasks"] });
        queryClient.invalidateQueries({ queryKey: ["visa", "active"] });
      }
    },
  });

  function submitMessage(e?: FormEvent, override?: string) {
    e?.preventDefault();
    const content = (override ?? draft).trim();
    if (!content || sendMessage.isPending) return;

    const nextMessage: ChatMessage = {
      id: crypto.randomUUID(),
      role: "user",
      content,
    };
    const nextMessages = [...apiMessages, { role: "user" as const, content }];

    setMessages((current) => [...current, nextMessage]);
    setDraft("");
    sendMessage.mutate(nextMessages);
  }

  return (
    <div className="mx-auto flex min-h-[calc(100vh-5rem)] max-w-md flex-col px-4 pt-6">
      <div className="mb-4 flex items-center gap-3">
        <MessageCircle className="h-6 w-6 text-primary" />
        <h1 className="text-2xl font-bold">Chat</h1>
      </div>

      <div className="flex-1 space-y-3 overflow-y-auto pb-4">
        {messages.map((message) => (
          <ChatBubble key={message.id} message={message} />
        ))}
        {sendMessage.isPending && (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Bot className="h-4 w-4 animate-pulse" />
            Thinking...
          </div>
        )}
      </div>

      {messages.length === 1 && (
        <div className="mb-3 flex flex-wrap gap-2">
          {STARTERS.map((starter) => (
            <button
              key={starter}
              type="button"
              onClick={() => submitMessage(undefined, starter)}
              className="rounded-full bg-muted px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-muted/80 hover:text-foreground"
            >
              {starter}
            </button>
          ))}
        </div>
      )}

      {sendMessage.isError && (
        <Card className="mb-3 border-destructive/40">
          <CardContent className="px-4 py-3 text-sm text-destructive">
            {sendMessage.error.message}
          </CardContent>
        </Card>
      )}

      <form onSubmit={submitMessage} className="sticky bottom-20 bg-background pb-4">
        <div className="flex gap-2">
          <Input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder="Message Japan Concierge"
            disabled={sendMessage.isPending}
            className="h-11"
          />
          <Button
            type="submit"
            size="icon"
            className="h-11 w-11 shrink-0"
            disabled={!draft.trim() || sendMessage.isPending}
            aria-label="Send message"
          >
            <Send className="h-4 w-4" />
          </Button>
        </div>
      </form>
    </div>
  );
}

function ChatBubble({ message }: { message: ChatMessage }) {
  const isUser = message.role === "user";
  return (
    <div className={cn("flex", isUser ? "justify-end" : "justify-start")}>
      <div
        className={cn(
          "max-w-[85%] rounded-lg px-3 py-2 text-sm leading-6",
          isUser
            ? "bg-primary text-primary-foreground"
            : "bg-muted text-foreground",
        )}
      >
        <p className="whitespace-pre-line break-words">{message.content}</p>
        {message.toolCalls && message.toolCalls.length > 0 && (
          <div className="mt-2 space-y-1">
            {message.toolCalls.map((call, index) => (
              <div
                key={`${call.name}-${index}`}
                className="flex items-center gap-1.5 text-xs text-muted-foreground"
              >
                {call.success ? (
                  <CheckCircle2 className="h-3.5 w-3.5 text-green-600" />
                ) : (
                  <Wrench className="h-3.5 w-3.5 text-destructive" />
                )}
                <Badge variant="outline" className="rounded px-1.5 py-0 text-[11px]">
                  {call.name}
                </Badge>
                <span className="break-words">{call.message}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
