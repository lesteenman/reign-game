import { useState, useCallback } from 'react';
import { Input, styled, Text, View } from 'tamagui';
import { PrimaryButton } from '@shared/components/Button';
import { ApiError } from '@shared/api';
import { useSetUsername } from '@shared/profile/useSetUsername';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const styledAny = styled as any;

const PromptBlock = styledAny(View, {
  name: 'ClaimUsernamePrompt',
  alignItems: 'center',
  gap: 12,
  width: '100%',
});

const PromptLabel = styledAny(Text, {
  name: 'ClaimUsernameLabel',
  fontSize: '$4',
  fontWeight: '600',
  color: '$body',
  textAlign: 'center',
  render: <p />,
  margin: 0,
});

const Row = styledAny(View, {
  name: 'ClaimUsernameRow',
  flexDirection: 'row',
  gap: 8,
  width: '100%',
  maxWidth: 360,
  alignItems: 'center',
});

const ErrorLine = styledAny(Text, {
  name: 'ClaimUsernameError',
  fontSize: '$3',
  fontWeight: '500',
  color: '$destructive',
  textAlign: 'center',
  render: <p />,
  margin: 0,
});

/**
 * Maps a failed set-username call to a single line of user-facing copy.
 * Status mapping mirrors the backend contract: 400 invalid format,
 * 409 taken, 422 not allowed (profanity/reserved).
 */
function errorCopy(err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.status) {
      case 400:
        return 'Use 3-20 letters, numbers, _ or -.';
      case 409:
        return 'That name is already taken. Try another.';
      case 422:
        return "That name isn't allowed. Try another.";
      default:
        return "Couldn't set that name right now. Try again.";
    }
  }
  return "Couldn't set that name right now. Try again.";
}

/**
 * Claim-a-name prompt shown after a signed-in, leaderboard-eligible daily
 * solve when the player has no username yet. Lets them set a leaderboard
 * display name inline, surfacing validation errors from the backend. On
 * success the hook calls user.reload(), which populates user.username and
 * flips the call site's visibility guard false (this component unmounts);
 * the durable confirmation lives at the call site, driven by user.username.
 *
 * The caller owns the eligibility gate (signed-in + leaderboard-eligible
 * + no username); this component only renders the form.
 */
export function ClaimUsernamePrompt() {
  const [value, setValue] = useState('');
  const mutation = useSetUsername();

  const handleSubmit = useCallback(() => {
    const trimmed = value.trim();
    if (trimmed.length === 0) return;
    mutation.mutate(trimmed);
  }, [value, mutation]);

  return (
    <PromptBlock data-testid="claim-username-prompt">
      <PromptLabel>Claim a name to appear on the board</PromptLabel>
      <Row>
        <Input
          flex={1}
          value={value}
          onChangeText={setValue}
          placeholder="your name"
          maxLength={20}
          autoCapitalize="none"
          disabled={mutation.isPending}
          data-testid="claim-username-input"
          aria-label="Leaderboard username"
        />
        <PrimaryButton
          onClick={handleSubmit}
          disabled={mutation.isPending || value.trim().length === 0}
          data-testid="claim-username-submit"
        >
          {mutation.isPending ? 'Saving…' : 'Claim'}
        </PrimaryButton>
      </Row>
      {mutation.isError && (
        <ErrorLine data-testid="claim-username-error">
          {errorCopy(mutation.error)}
        </ErrorLine>
      )}
    </PromptBlock>
  );
}
