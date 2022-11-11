# Feature Tree

Feature flags are stored as n-ary trees, with each non-root node having a rule and an optional default value.
The root node has no rule, and has a non-optional default value.

Feature evaluation traverses the n-ary in depth-first search fashion, starting at the root, and ending as soon as a leaf node evaluates to true or there are no further paths to traverse. 
The algorithm returns the last non-null value it visited.

```
Feature Name: illustration
Description: Complex feature tree to illustrate node paths


                                            *Root*
                                            Path: []
                                            Value: 12
                                            /   |   \
                           -----------------    |    -------------------
                          /                     |                        \
                    Rule: a == 1            Rule: a > 10             Rule: a > 5
                    Path: [0]               Path: [1]                Path: [2]
                    Value: 38               Value: NULL              Value: 23
                        /                       |
             -----------                        |
            /                                   |
    Rule: x IN ["a", "b"]                   Rule: x == "c"
    Path: [0,0]                             Path: [1,0]
    Value: 108                              Value: 21
   

*** Tests ***

Context: {}
Value: 12
Path: []

Context: {"a": 1}
Value: 38
Path: [0]

Context: {"a": 11}
Value: 12
Path: []

Context: {"a": 11, "x": "c"}
Value: 21
Path: [1,0]

Context: {"a": 8}
Value: 23
Path: [2]

Context: {"a": 8}
Value: 108
Path: [0,0]
```