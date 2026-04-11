"use client";

import { useCallback } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

export function buildQueryParamHref(
  pathname: string,
  search: string,
  key: string,
  value: string,
): string {
  const params = new URLSearchParams(search);
  if (value) {
    params.set(key, value);
  } else {
    params.delete(key);
  }
  const query = params.toString();
  return query ? `${pathname}?${query}` : pathname;
}

export function useQueryParamSelection(key: string) {
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const value = searchParams.get(key) ?? "";

  const setValue = useCallback(
    (nextValue: string, { replace = false }: { replace?: boolean } = {}) => {
      const href = buildQueryParamHref(
        pathname,
        searchParams.toString(),
        key,
        nextValue,
      );
      if (replace) {
        router.replace(href, { scroll: false });
      } else {
        router.push(href, { scroll: false });
      }
    },
    [key, pathname, router, searchParams],
  );

  return [value, setValue] as const;
}
