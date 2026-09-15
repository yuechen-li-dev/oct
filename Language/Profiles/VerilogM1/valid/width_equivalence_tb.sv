module width_equivalence_tb;
    logic signed [63:0] left;
    logic signed [63:0] right;
    logic signed [63:0] addResult;
    logic signed [63:0] subtractResult;
    logic signed [63:0] multiplyResult;
    logic signed [63:0] negateResult;
    logic lessResult;

    AddEdge addDut(.left(left), .right(right), .Result(addResult));
    SubtractEdge subtractDut(.left(left), .right(right), .Result(subtractResult));
    MultiplyEdge multiplyDut(.left(left), .right(right), .Result(multiplyResult));
    NegateEdge negateDut(.value(left), .Result(negateResult));
    LessEdge lessDut(.left(left), .right(right), .Result(lessResult));

    initial begin
        left = 64'sh7fff_ffff_ffff_ffff;
        right = 64'sd1;
        #1;
        if (addResult !== 64'sh8000_0000_0000_0000) $fatal(1, "addition did not wrap at 64 bits");
        if (subtractResult !== 64'sh7fff_ffff_ffff_fffe) $fatal(1, "subtraction mismatch");

        left = -64'sd3;
        right = 64'sd7;
        #1;
        if (multiplyResult !== -64'sd21) $fatal(1, "signed multiplication mismatch");
        if (negateResult !== 64'sd3) $fatal(1, "signed negation mismatch");
        if (lessResult !== 1'b1) $fatal(1, "signed comparison mismatch");

        left = 64'sh8000_0000_0000_0000;
        #1;
        if (negateResult !== 64'sh8000_0000_0000_0000) $fatal(1, "minimum Int negation did not wrap");

        $display("PASS signed 64-bit arithmetic and overflow vectors");
        $finish;
    end
endmodule
