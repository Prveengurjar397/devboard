import { useEffect, useState } from 'react';
import { api } from '../api/client';
import { useOrganizationContext } from '../hooks/useOrganizationContext';

export function ChatbotPage() {
  const { organizations, currentOrganization } = useOrganizationContext();

  const [organizationId, setOrganizationId] = useState('');
  const [conversations, setConversations] = useState([]);
  const [conversationId, setConversationId] = useState(null);
  const [messages, setMessages] = useState([]);
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (currentOrganization?.id) {
      setOrganizationId(String(currentOrganization.id));
    }
  }, [currentOrganization]);

  async function loadConversations(orgId) {
    if (!orgId) return;
    try {
      const result = await api.get(
        `/api/chatbot/conversations?organization_id=${orgId}`
      );
      setConversations(result.conversations || []);
    } catch (err) {
      setError(err.message);
    }
  }

  useEffect(() => {
    if (organizationId) loadConversations(organizationId);
  }, [organizationId]);

  async function openConversation(id) {
    try {
      setError('');
      const result = await api.get(
        `/api/chatbot/conversations/${id}?organization_id=${organizationId}`
      );
      setConversationId(id);
      setMessages(result.messages || []);
    } catch (err) {
      setError(err.message);
    }
  }

  async function sendMessage(event) {
    event.preventDefault();
    const text = message.trim();
    if (!text || loading || !organizationId) return;

    setMessage('');
    setError('');
    setLoading(true);

    const temporaryMessage = {
      role: 'user',
      content: text,
    };

    setMessages((previous) => [...previous, temporaryMessage]);

    try {
      const result = await api.post('/api/chatbot/message', {
        organization_id: Number(organizationId),
        conversation_id: conversationId || 0,
        message: text,
      });

      setConversationId(result.conversation_id);
      setMessages((previous) => [
        ...previous,
        {
          role: 'assistant',
          content: result.reply,
          provider: result.provider,
        },
      ]);

      await loadConversations(organizationId);
    } catch (err) {
      setError(err.message);
      setMessages((previous) => previous.slice(0, -1));
      setMessage(text);
    } finally {
      setLoading(false);
    }
  }

  function newConversation() {
    setConversationId(null);
    setMessages([]);
    setError('');
  }

  return (
    <div className="h-full min-h-[600px] flex flex-col lg:flex-row gap-4">
      <aside className="w-full lg:w-64 shrink-0 rounded-xl border border-ink-100 dark:border-white/10 bg-white dark:bg-[#1c1c1f] p-3">
        <button
          onClick={newConversation}
          className="w-full rounded-lg bg-accent text-white px-4 py-3 text-sm font-medium"
        >
          + New conversation
        </button>

        <div className="mt-4">
          <label className="text-xs text-ink-400">Workspace</label>
          <select
            value={organizationId}
            onChange={(e) => {
              setOrganizationId(e.target.value);
              newConversation();
            }}
            className="mt-1 w-full rounded-lg border border-ink-200 dark:border-white/10 bg-transparent p-2 text-sm"
          >
            {organizations.map((org) => (
              <option key={org.id} value={org.id}>
                {org.name}
              </option>
            ))}
          </select>
        </div>

        <h3 className="mt-6 mb-2 text-xs font-semibold uppercase tracking-wide text-ink-400">
          Recent conversations
        </h3>

        <div className="space-y-1">
          {conversations.map((conversation) => (
            <button
              key={conversation.id}
              onClick={() => openConversation(conversation.id)}
              className={`w-full text-left rounded-lg px-3 py-2 text-sm hover:bg-ink-50 dark:hover:bg-white/5 ${
                Number(conversationId) === Number(conversation.id)
                  ? 'bg-accent-subtle text-accent-strong'
                  : ''
              }`}
            >
              <span className="block truncate">{conversation.title}</span>
            </button>
          ))}
        </div>
      </aside>

      <section className="flex-1 min-w-0 flex flex-col rounded-xl border border-ink-100 dark:border-white/10 bg-white dark:bg-[#1c1c1f] overflow-hidden">
        <header className="p-4 border-b border-ink-100 dark:border-white/10">
          <h1 className="font-semibold text-lg">OpsMind AI</h1>
          <p className="text-xs text-ink-400 mt-1">
            DevOps workspace assistant
          </p>
        </header>

        <div className="flex-1 min-h-[350px] overflow-y-auto p-4 space-y-4">
          {messages.length === 0 && (
            <div className="h-full min-h-[300px] flex flex-col items-center justify-center text-center">
              <div className="text-3xl mb-3">✦</div>
              <h2 className="font-semibold">How can I help?</h2>
              <p className="text-sm text-ink-400 mt-2 max-w-md">
                Ask about your company's incidents, tasks, projects,
                container errors or DevOps troubleshooting.
              </p>
            </div>
          )}

          {messages.map((item, index) => (
            <div
              key={`${index}-${item.role}`}
              className={`flex ${item.role === 'user' ? 'justify-end' : 'justify-start'}`}
            >
              <div
                className={`max-w-[85%] rounded-xl px-4 py-3 text-sm whitespace-pre-wrap ${
                  item.role === 'user'
                    ? 'bg-accent text-white'
                    : 'bg-ink-50 dark:bg-white/5'
                }`}
              >
                {item.content}
                {item.provider && (
                  <div className="mt-2 text-[10px] opacity-60">
                    Provider: {item.provider}
                  </div>
                )}
              </div>
            </div>
          ))}

          {loading && (
            <div className="text-sm text-ink-400">
              OpsMind AI is thinking...
            </div>
          )}
        </div>

        {error && (
          <div className="mx-4 mb-2 text-sm text-red-500">{error}</div>
        )}

        <form
          onSubmit={sendMessage}
          className="p-3 border-t border-ink-100 dark:border-white/10 flex gap-2"
        >
          <input
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            placeholder="Ask OpsMind AI..."
            maxLength={8000}
            className="flex-1 min-w-0 rounded-lg border border-ink-200 dark:border-white/10 bg-transparent px-4 py-3 text-sm outline-none"
          />
          <button
            type="submit"
            disabled={loading || !message.trim() || !organizationId}
            className="rounded-lg bg-accent text-white px-5 py-3 text-sm font-medium disabled:opacity-40"
          >
            Send
          </button>
        </form>
      </section>
    </div>
  );
}
