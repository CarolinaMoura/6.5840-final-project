/* Copyright (c) 2021-25 MIT 6.102/6.031 course staff, all rights reserved.
 * Redistribution of original or derived work requires permission of course staff.
 */
type Resolver<T> = (value: T | PromiseLike<T>) => void;
type Rejector = (reason: Error) => void;

/** Deferred represents a promise plus operations to resolve or reject it. */
export class Deferred<T> {
  /** The promise. */
  public readonly promise: Promise<T>;

  /** Mutator: fulfill the promise with a value of type T. */
  public readonly resolve: Resolver<T>;

  /** Mutator: reject the promise with an Error value. */
  public readonly reject: Rejector;

  /** Make a new Deferred. */
  public constructor() {
    const { promise, resolve, reject } = Promise.withResolvers<T>();
    this.promise = promise;
    this.resolve = resolve;
    this.reject = reject;
  }
}
