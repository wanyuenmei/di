#!/usr/bin/python

import sys
import os


def checkFile(filepath):
    with open(filepath, 'r') as f:
        offendingLines = [
            (i, l)
            for i, l
            in enumerate(f)
            if len(l) > 89
        ]
        if (len(offendingLines) > 0):
            print(
                    "{} contains {} lines over 89 chars long".format(
                        filepath,
                        len(offendingLines)
                    )
                 )
            for i, l in offendingLines:
                print("{} ({}): {}".format(i, len(l), l))
        return len(offendingLines)

totalOffending = sum(checkFile(f) for f in sys.argv)

if(totalOffending > 0):
    print("{} total offending lines".format(totalOffending))
