module equivalence_tb;
    logic [127:0] sensor;
    logic [65:0] command;
    logic [128:0] result;

    MotorCommand dut(.sensor(sensor), .command(command), .Result(result));

    task automatic expect_result(
        input logic signed [63:0] requested,
        input logic signed [63:0] safe,
        input logic limited
    );
        #1;
        if ($signed(result[63:0]) !== requested ||
            $signed(result[127:64]) !== safe ||
            result[128] !== limited) begin
            $fatal(1, "result mismatch: requested=%0d safe=%0d limited=%0d", $signed(result[63:0]), $signed(result[127:64]), result[128]);
        end
    endtask

    initial begin
        sensor = {64'sd10, 64'sd7};

        command = {64'sd0, 2'd0};
        expect_result(64'sd7, 64'sd7, 1'b0);

        command = {64'sd8, 2'd1};
        expect_result(64'sd15, 64'sd10, 1'b1);

        command = {-64'sd2, 2'd2};
        expect_result(-64'sd14, -64'sd10, 1'b1);

        $display("PASS reference vectors: Hold, Offset, Scale");
        $finish;
    end
endmodule
